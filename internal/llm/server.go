package llm

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/alexandr/audio-text-manager-desktop/internal/sidecar"
)

const defaultPort = 18765

type Server struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	baseURL   string
	modelPath string
	bin       string
	stderr    bytes.Buffer
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) BaseURL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.baseURL
}

func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
}

func (s *Server) stopLocked() {
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_, _ = s.cmd.Process.Wait()
	}
	s.cmd = nil
	s.baseURL = ""
	s.modelPath = ""
	s.bin = ""
	s.stderr.Reset()
}

// Ensure starts llama-server if needed and waits until /health succeeds.
func (s *Server) Ensure(ctx context.Context, bin, modelPath string) (string, error) {
	if bin == "" {
		return "", fmt.Errorf("llama-server not found. Bundle llama.cpp (make fetch-runtime) or set llama_bin")
	}
	if modelPath == "" {
		return "", fmt.Errorf("llama GGUF model not found")
	}
	s.mu.Lock()
	if s.cmd != nil && s.cmd.Process != nil && s.bin == bin && s.modelPath == modelPath && s.baseURL != "" {
		url := s.baseURL
		s.mu.Unlock()
		if err := pingHealth(ctx, url); err == nil {
			return url, nil
		}
		s.mu.Lock()
		s.stopLocked()
	}
	s.stopLocked()

	port, err := freePort(defaultPort)
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	cmd := exec.Command(bin,
		"-m", modelPath,
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", port),
		"-c", "16384",
		"--jinja",
	)
	sidecar.Prep(cmd)
	s.stderr.Reset()
	cmd.Stderr = &s.stderr
	if err := cmd.Start(); err != nil {
		s.mu.Unlock()
		return "", fmt.Errorf("llama-server: %w", err)
	}
	s.cmd = cmd
	s.bin = bin
	s.modelPath = modelPath
	s.baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	url := s.baseURL
	s.mu.Unlock()

	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			s.Stop()
			return "", err
		}
		if err := pingHealth(ctx, url); err == nil {
			return url, nil
		}
		s.mu.Lock()
		alive := s.cmd != nil && s.cmd.ProcessState == nil
		errTail := s.stderr.String()
		s.mu.Unlock()
		if !alive {
			s.Stop()
			return "", fmt.Errorf("llama-server exited: %s", trimTail(errTail))
		}
		select {
		case <-ctx.Done():
			s.Stop()
			return "", ctx.Err()
		case <-time.After(400 * time.Millisecond):
		}
	}
	tail := s.stderr.String()
	s.Stop()
	return "", fmt.Errorf("llama-server did not become ready: %s", trimTail(tail))
}

func pingHealth(ctx context.Context, baseURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("health HTTP %d", resp.StatusCode)
	}
	return nil
}

func freePort(preferred int) (int, error) {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", preferred))
	if err == nil {
		defer l.Close()
		return preferred, nil
	}
	l, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("no free port for llama-server: %w", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func trimTail(s string) string {
	s = string([]rune(s))
	if len(s) > 800 {
		s = s[len(s)-800:]
	}
	return s
}
