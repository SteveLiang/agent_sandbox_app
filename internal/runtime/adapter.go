package runtime

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var safeID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{2,63}$`)

type Adapter struct {
	ThinPool string
	VMRoot   string
}

type CommandResult struct {
	Command string `json:"command"`
	Output  string `json:"output"`
}

type VMResult struct {
	SandboxID  string `json:"sandbox_id"`
	PID        int    `json:"pid"`
	SocketPath string `json:"socket_path"`
	RootFSPath string `json:"rootfs_path"`
	StateDir   string `json:"state_dir"`
}

type VMStatus struct {
	SandboxID      string `json:"sandbox_id"`
	StateDir       string `json:"state_dir"`
	PIDPath        string `json:"pid_path"`
	SocketPath     string `json:"socket_path"`
	LogPath        string `json:"log_path"`
	RootFSPath     string `json:"rootfs_path"`
	PID            int    `json:"pid"`
	PIDFileExists  bool   `json:"pid_file_exists"`
	SocketExists   bool   `json:"socket_exists"`
	ProcessRunning bool   `json:"process_running"`
}

func (a Adapter) CreateSandboxVolume(ctx context.Context, sandboxID string, sizeGB int) (CommandResult, error) {
	if sizeGB <= 0 {
		return CommandResult{}, errors.New("size_gb must be > 0")
	}
	if err := validateID(sandboxID); err != nil {
		return CommandResult{}, err
	}
	name := "sbx-" + sandboxID
	cmd := []string{"lvcreate", "-V", fmt.Sprintf("%dG", sizeGB), "-T", a.ThinPool, "-n", name}
	return runCommand(ctx, cmd)
}

func (a Adapter) SnapshotSandboxVolume(ctx context.Context, sandboxID, snapshotID string) (CommandResult, error) {
	if err := validateID(sandboxID); err != nil {
		return CommandResult{}, err
	}
	if err := validateID(snapshotID); err != nil {
		return CommandResult{}, err
	}
	origin := volumePath(a.ThinPool, "sbx-"+sandboxID)
	cmd := []string{"lvcreate", "-s", "-n", "snap-" + snapshotID, origin}
	return runCommand(ctx, cmd)
}

func (a Adapter) RestoreSnapshotToSandbox(ctx context.Context, snapshotID, sandboxID string) (CommandResult, error) {
	if err := validateID(snapshotID); err != nil {
		return CommandResult{}, err
	}
	if err := validateID(sandboxID); err != nil {
		return CommandResult{}, err
	}
	origin := volumePath(a.ThinPool, "snap-"+snapshotID)
	cmd := []string{"lvcreate", "-s", "-n", "sbx-" + sandboxID, origin}
	return runCommand(ctx, cmd)
}

func (a Adapter) DeleteSandboxVolume(ctx context.Context, sandboxID string) (CommandResult, error) {
	if err := validateID(sandboxID); err != nil {
		return CommandResult{}, err
	}
	path := volumePath(a.ThinPool, "sbx-"+sandboxID)
	cmd := []string{"lvremove", "-y", path}
	return runCommand(ctx, cmd)
}

func (a Adapter) DeleteSnapshotVolume(ctx context.Context, snapshotID string) (CommandResult, error) {
	if err := validateID(snapshotID); err != nil {
		return CommandResult{}, err
	}
	path := volumePath(a.ThinPool, "snap-"+snapshotID)
	cmd := []string{"lvremove", "-y", path}
	return runCommand(ctx, cmd)
}

func (a Adapter) StartSandboxVM(
	ctx context.Context,
	sandboxID, kernelImage string,
	vcpuCount, memMiB int,
	hostTap, guestMAC string,
) (VMResult, error) {
	if err := validateID(sandboxID); err != nil {
		return VMResult{}, err
	}
	if kernelImage == "" {
		return VMResult{}, errors.New("kernel_image is required")
	}
	if vcpuCount <= 0 || memMiB <= 0 {
		return VMResult{}, errors.New("vcpu_count and mem_mib must be > 0")
	}

	rootFS := volumePath(a.ThinPool, "sbx-"+sandboxID)
	if _, err := os.Stat(rootFS); err != nil {
		return VMResult{}, fmt.Errorf("rootfs path missing: %s (%w)", rootFS, err)
	}

	// Thin snapshots can be marked activation-skip; clear it and force activation.
	if _, err := runCommand(ctx, []string{"lvchange", "--setactivationskip", "n", rootFS}); err != nil {
		return VMResult{}, err
	}
	if _, err := runCommand(ctx, []string{"lvchange", "-ay", rootFS}); err != nil {
		return VMResult{}, err
	}

	stateDir := filepath.Join(a.VMRoot, "sandboxes", sandboxID)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return VMResult{}, fmt.Errorf("mkdir state dir: %w", err)
	}
	socketPath := filepath.Join(stateDir, "firecracker.sock")
	pidPath := filepath.Join(stateDir, "firecracker.pid")
	logPath := filepath.Join(stateDir, "firecracker.log")
	_ = os.Remove(socketPath)

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return VMResult{}, fmt.Errorf("open log file: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command("firecracker", "--api-sock", socketPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return VMResult{}, fmt.Errorf("start firecracker: %w", err)
	}
	cleanupNeeded := true
	defer func() {
		if cleanupNeeded {
			_ = cmd.Process.Kill()
			_ = os.Remove(socketPath)
			_ = os.Remove(pidPath)
		}
	}()

	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		return VMResult{}, fmt.Errorf("write pid file: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := waitForSocket(waitCtx, socketPath); err != nil {
		return VMResult{}, err
	}

	if err := putFC(
		waitCtx,
		socketPath,
		"/machine-config",
		map[string]any{"vcpu_count": vcpuCount, "mem_size_mib": memMiB, "smt": false},
	); err != nil {
		return VMResult{}, err
	}
	if err := putFC(
		waitCtx,
		socketPath,
		"/boot-source",
		map[string]any{
			"kernel_image_path": kernelImage,
			"boot_args":         "console=ttyS0 reboot=k panic=1 pci=off root=/dev/vda rw",
		},
	); err != nil {
		return VMResult{}, err
	}
	if err := putFC(
		waitCtx,
		socketPath,
		"/drives/rootfs",
		map[string]any{
			"drive_id":       "rootfs",
			"path_on_host":   rootFS,
			"is_root_device": true,
			"is_read_only":   false,
		},
	); err != nil {
		return VMResult{}, err
	}
	if hostTap != "" {
		if guestMAC == "" {
			guestMAC = deterministicGuestMAC(sandboxID)
		}
		if err := putFC(
			waitCtx,
			socketPath,
			"/network-interfaces/eth0",
			map[string]any{
				"iface_id":      "eth0",
				"host_dev_name": hostTap,
				"guest_mac":     guestMAC,
			},
		); err != nil {
			return VMResult{}, err
		}
	}
	if err := putFC(waitCtx, socketPath, "/actions", map[string]any{"action_type": "InstanceStart"}); err != nil {
		return VMResult{}, err
	}
	cleanupNeeded = false

	return VMResult{
		SandboxID:  sandboxID,
		PID:        cmd.Process.Pid,
		SocketPath: socketPath,
		RootFSPath: rootFS,
		StateDir:   stateDir,
	}, nil
}

func (a Adapter) StopSandboxVM(_ context.Context, sandboxID string) (CommandResult, error) {
	if err := validateID(sandboxID); err != nil {
		return CommandResult{}, err
	}
	stateDir := filepath.Join(a.VMRoot, "sandboxes", sandboxID)
	pidPath := filepath.Join(stateDir, "firecracker.pid")
	raw, err := os.ReadFile(pidPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_ = os.Remove(filepath.Join(stateDir, "firecracker.sock"))
			return CommandResult{
				Command: "noop",
				Output:  "sandbox " + sandboxID + " already stopped (pid file not found)",
			}, nil
		}
		return CommandResult{}, fmt.Errorf("read pid file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return CommandResult{}, fmt.Errorf("parse pid: %w", err)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return CommandResult{}, fmt.Errorf("find process: %w", err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return CommandResult{}, fmt.Errorf("stop process: %w", err)
	}
	_ = os.Remove(filepath.Join(stateDir, "firecracker.sock"))
	_ = os.Remove(pidPath)
	return CommandResult{
		Command: "kill -TERM " + strconv.Itoa(pid),
		Output:  "stopped sandbox " + sandboxID,
	}, nil
}

func (a Adapter) SandboxStatus(_ context.Context, sandboxID string) (VMStatus, error) {
	if err := validateID(sandboxID); err != nil {
		return VMStatus{}, err
	}
	stateDir := filepath.Join(a.VMRoot, "sandboxes", sandboxID)
	pidPath := filepath.Join(stateDir, "firecracker.pid")
	socketPath := filepath.Join(stateDir, "firecracker.sock")
	logPath := filepath.Join(stateDir, "firecracker.log")
	rootFS := volumePath(a.ThinPool, "sbx-"+sandboxID)

	status := VMStatus{
		SandboxID:  sandboxID,
		StateDir:   stateDir,
		PIDPath:    pidPath,
		SocketPath: socketPath,
		LogPath:    logPath,
		RootFSPath: rootFS,
	}

	if _, err := os.Stat(pidPath); err == nil {
		status.PIDFileExists = true
		raw, readErr := os.ReadFile(pidPath)
		if readErr == nil {
			if pid, convErr := strconv.Atoi(strings.TrimSpace(string(raw))); convErr == nil {
				status.PID = pid
				if processRunning(pid) {
					status.ProcessRunning = true
				}
			}
		}
	}
	if _, err := os.Stat(socketPath); err == nil {
		status.SocketExists = true
	}

	return status, nil
}

func (a Adapter) CheckDependencies(ctx context.Context) (map[string]bool, error) {
	deps := []string{"lvcreate", "lvremove", "lvs", "firecracker"}
	out := make(map[string]bool, len(deps))
	for _, dep := range deps {
		cmd := exec.CommandContext(ctx, "sh", "-c", "command -v "+dep+" >/dev/null 2>&1")
		err := cmd.Run()
		out[dep] = err == nil
	}
	return out, nil
}

func validateID(id string) error {
	if !safeID.MatchString(id) {
		return fmt.Errorf("invalid id %q: must match %s", id, safeID.String())
	}
	return nil
}

func volumePath(thinPool, lvName string) string {
	parts := strings.SplitN(thinPool, "/", 2)
	if len(parts) != 2 {
		return "/dev/" + lvName
	}
	return fmt.Sprintf("/dev/%s/%s", parts[0], lvName)
}

func runCommand(ctx context.Context, args []string) (CommandResult, error) {
	if len(args) == 0 {
		return CommandResult{}, errors.New("empty command")
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	res := CommandResult{
		Command: strings.Join(args, " "),
		Output:  strings.TrimSpace(out.String()),
	}
	if err != nil {
		b, _ := json.Marshal(res)
		return res, fmt.Errorf("command failed: %v (%s)", err, string(b))
	}
	return res, nil
}

func putFC(ctx context.Context, socketPath, path string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, "http://localhost"+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("firecracker %s failed: status=%d body=%s", path, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func waitForSocket(ctx context.Context, socketPath string) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("firecracker socket not ready: %w", ctx.Err())
		case <-ticker.C:
			if _, err := os.Stat(socketPath); err == nil {
				return nil
			}
		}
	}
}

func processRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}

func deterministicGuestMAC(sandboxID string) string {
	sum := sha1.Sum([]byte(sandboxID))
	// Locally administered unicast MAC: 02:xx:xx:xx:xx:xx
	return fmt.Sprintf("02:%02x:%02x:%02x:%02x:%02x", sum[0], sum[1], sum[2], sum[3], sum[4])
}
