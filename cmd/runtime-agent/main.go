package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"microvm-sandbox-mvp/internal/runtime"
)

type server struct {
	adapter runtime.Adapter
}

type volumeReq struct {
	SandboxID  string `json:"sandbox_id"`
	SnapshotID string `json:"snapshot_id"`
	SizeGB     int    `json:"size_gb"`
}

type restoreReq struct {
	SnapshotID string `json:"snapshot_id"`
	SandboxID  string `json:"sandbox_id"`
}

type startReq struct {
	SandboxID   string `json:"sandbox_id"`
	KernelImage string `json:"kernel_image"`
	VCPUCount   int    `json:"vcpu_count"`
	MemMiB      int    `json:"mem_mib"`
}

func main() {
	addr := envOrDefault("RUNTIME_ADDR", ":8081")
	thinPool := envOrDefault("THIN_POOL", "microvm-vg/sandbox-thinpool")
	vmRoot := envOrDefault("MICROVM_ROOT", "/var/lib/microvm")

	s := &server{adapter: runtime.Adapter{ThinPool: thinPool, VMRoot: vmRoot}}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("POST /v1/sandboxes", s.handleCreateSandbox)
	mux.HandleFunc("DELETE /v1/sandboxes", s.handleDeleteSandbox)
	mux.HandleFunc("GET /v1/sandboxes/status", s.handleSandboxStatus)
	mux.HandleFunc("POST /v1/sandboxes/start", s.handleStartSandbox)
	mux.HandleFunc("POST /v1/sandboxes/stop", s.handleStopSandbox)
	mux.HandleFunc("POST /v1/snapshots", s.handleSnapshot)
	mux.HandleFunc("DELETE /v1/snapshots", s.handleDeleteSnapshot)
	mux.HandleFunc("POST /v1/restores", s.handleRestore)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("runtime-agent listening on %s (thin_pool=%s)", addr, thinPool)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}
}

func (s *server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	deps, err := s.adapter.CheckDependencies(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "dependencies": deps})
}

func (s *server) handleCreateSandbox(w http.ResponseWriter, r *http.Request) {
	var req volumeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	res, err := s.adapter.CreateSandboxVolume(ctx, req.SandboxID, req.SizeGB)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "result": res})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "result": res})
}

func (s *server) handleDeleteSandbox(w http.ResponseWriter, r *http.Request) {
	sandboxID := r.URL.Query().Get("sandbox_id")
	if sandboxID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sandbox_id is required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	res, err := s.adapter.DeleteSandboxVolume(ctx, sandboxID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "result": res})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": res})
}

func (s *server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	var req volumeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	res, err := s.adapter.SnapshotSandboxVolume(ctx, req.SandboxID, req.SnapshotID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "result": res})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "result": res})
}

func (s *server) handleStartSandbox(w http.ResponseWriter, r *http.Request) {
	var req startReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.KernelImage == "" {
		req.KernelImage = envOrDefault("KERNEL_IMAGE", "/var/lib/microvm/images/vmlinux.bin")
	}
	if req.VCPUCount == 0 {
		req.VCPUCount = envIntOrDefault("DEFAULT_VCPU", 2)
	}
	if req.MemMiB == 0 {
		req.MemMiB = envIntOrDefault("DEFAULT_MEM_MIB", 1024)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	res, err := s.adapter.StartSandboxVM(ctx, req.SandboxID, req.KernelImage, req.VCPUCount, req.MemMiB)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "result": res})
}

func (s *server) handleSandboxStatus(w http.ResponseWriter, r *http.Request) {
	sandboxID := r.URL.Query().Get("sandbox_id")
	if sandboxID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sandbox_id is required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	status, err := s.adapter.SandboxStatus(ctx, sandboxID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": status})
}

func (s *server) handleStopSandbox(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SandboxID string `json:"sandbox_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	res, err := s.adapter.StopSandboxVM(ctx, req.SandboxID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": res})
}

func (s *server) handleDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshotID := r.URL.Query().Get("snapshot_id")
	if snapshotID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "snapshot_id is required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	res, err := s.adapter.DeleteSnapshotVolume(ctx, snapshotID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "result": res})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": res})
}

func (s *server) handleRestore(w http.ResponseWriter, r *http.Request) {
	var req restoreReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	res, err := s.adapter.RestoreSnapshotToSandbox(ctx, req.SnapshotID, req.SandboxID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "result": res})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "result": res})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func envOrDefault(key, dflt string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return dflt
}

func envIntOrDefault(key string, dflt int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return dflt
}
