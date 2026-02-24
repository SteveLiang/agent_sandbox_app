package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
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
