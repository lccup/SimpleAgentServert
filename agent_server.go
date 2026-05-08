package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	port        string
	apiKey      string
	allowList   []string
	allowListMu sync.RWMutex
	version     = "1.0.0"
)

type ExecuteRequest struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type ExecuteResponse struct {
	Success   bool   `json:"success"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  int    `json:"exit_code"`
	Error     string `json:"error,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Timestamp int64  `json:"timestamp"`
}

type ErrorResponse struct {
	Error     string `json:"error"`
	Timestamp int64  `json:"timestamp"`
}

func main() {
	flag.StringVar(&port, "port", "8080", "Server port")
	flag.StringVar(&apiKey, "apikey", "", "API Key for authentication")
	allowListStr := flag.String("allowlist", "", "Comma-separated list of allowed commands (empty = allow all)")
	flag.Parse()

	if apiKey == "" {
		apiKey = os.Getenv("AGENT_API_KEY")
	}

	if apiKey == "" {
		log.Fatal("API key is required. Set -apikey flag or AGENT_API_KEY environment variable")
	}

	if *allowListStr != "" {
		allowList = strings.Split(*allowListStr, ",")
		log.Printf("Command allowlist enabled: %v", allowList)
	}

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/execute", authMiddleware(executeHandler))

	log.Printf("SimpleAgentServert starting on :%s", port)
	log.Printf("Version: %s", version)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		key := r.Header.Get("X-API-Key")
		if key == "" {
			key = r.URL.Query().Get("apikey")
		}

		if key != apiKey {
			log.Printf("[AUTH] Unauthorized access attempt from %s", r.RemoteAddr)
			writeError(w, http.StatusUnauthorized, "Invalid or missing API key")
			return
		}

		log.Printf("[AUTH] Authorized request from %s", r.RemoteAddr)
		next(w, r)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	resp := HealthResponse{
		Status:    "ok",
		Version:   version,
		Timestamp: time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func executeHandler(w http.ResponseWriter, r *http.Request) {
	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON request")
		return
	}

	if req.Command == "" {
		writeError(w, http.StatusBadRequest, "Command is required")
		return
	}

	allowListMu.RLock()
	isAllowed := isCommandAllowed(req.Command)
	allowListMu.RUnlock()

	if !isAllowed {
		log.Printf("[BLOCK] Command not in allowlist: %s", req.Command)
		writeError(w, http.StatusForbidden, "Command not allowed")
		return
	}

	timeout := req.Timeout
	if timeout <= 0 || timeout > 300 {
		timeout = 30
	}

	log.Printf("[EXEC] Executing: %s (timeout: %ds)", req.Command, timeout)

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("bash", "-c", req.Command)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		resp := ExecuteResponse{
			Success:   err == nil,
			Stdout:    stdout.String(),
			Stderr:    stderr.String(),
			ExitCode:  getExitCode(err),
			Timestamp: time.Now().Unix(),
		}
		if err != nil {
			resp.Error = err.Error()
		}

		log.Printf("[EXEC] Completed: exit=%d, stdout=%d bytes, stderr=%d bytes",
			resp.ExitCode, len(stdout.Bytes()), len(stderr.Bytes()))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	case <-time.After(time.Duration(timeout) * time.Second):
		cmd.Process.Kill()
		<-done

		resp := ExecuteResponse{
			Success:   false,
			Stdout:    stdout.String(),
			Stderr:    "Command timed out",
			ExitCode:  -1,
			Error:     fmt.Sprintf("Command execution timed out after %d seconds", timeout),
			Timestamp: time.Now().Unix(),
		}

		log.Printf("[EXEC] Timeout: %s (%ds)", req.Command, timeout)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func isCommandAllowed(cmd string) bool {
	if len(allowList) == 0 {
		return true
	}

	cmdLower := strings.ToLower(cmd)
	for _, allowed := range allowList {
		allowed = strings.TrimSpace(strings.ToLower(allowed))
		if strings.HasPrefix(cmdLower, allowed) {
			return true
		}
	}
	return false
}

func getExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := ErrorResponse{
		Error:     message,
		Timestamp: time.Now().Unix(),
	}
	json.NewEncoder(w).Encode(resp)
}
