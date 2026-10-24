package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// LifecycleManager manages server readiness and Docker Compose lifecycle for test environments.
type LifecycleManager struct {
	composeFile string
	targetURL   string
	timeout     time.Duration

	// dockerCmd holds the command prefix to execute compose operations (e.g. ["docker", "compose"]).
	// Tests may inject custom command prefixes to mock Docker CLI execution.
	dockerCmd []string
}

// NewLifecycleManager constructs a LifecycleManager for existing server checks or Docker Compose environments.
func NewLifecycleManager(composeFile, targetURL string, timeoutSec int) *LifecycleManager {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	return &LifecycleManager{
		composeFile: strings.TrimSpace(composeFile),
		targetURL:   strings.TrimSpace(targetURL),
		timeout:     time.Duration(timeoutSec) * time.Second,
	}
}

// SetDockerCmd overrides the docker CLI binary path and subcommand (used for testing mocks).
func (m *LifecycleManager) SetDockerCmd(cmd []string) {
	m.dockerCmd = cmd
}

// Up initiates the server environment and waits until targetURL is healthy.
// In existing-server mode (composeFile == ""), it polls targetURL and returns a no-op teardown.
// In Docker Compose mode, it runs `docker compose -f <file> up -d`, verifies readiness, and
// returns a teardown function that executes `docker compose -f <file> down`.
func (m *LifecycleManager) Up(ctx context.Context) (teardown func(), err error) {
	if m.composeFile == "" {
		if err := WaitForReady(ctx, m.targetURL, m.timeout); err != nil {
			return nil, fmt.Errorf("existing server check: %w", err)
		}
		return func() {}, nil
	}

	cleanPath := filepath.Clean(m.composeFile)
	if _, err := os.Stat(cleanPath); err != nil {
		return nil, fmt.Errorf("docker compose file not found: %w", err)
	}

	cmdPrefix, err := m.resolveDockerComposeCmd()
	if err != nil {
		return nil, err
	}

	upArgs := append([]string{}, cmdPrefix[1:]...)
	upArgs = append(upArgs, "-f", cleanPath, "up", "-d")

	upCmd := exec.CommandContext(ctx, cmdPrefix[0], upArgs...)
	out, err := upCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker compose up failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	teardown = func() {
		downArgs := append([]string{}, cmdPrefix[1:]...)
		downArgs = append(downArgs, "-f", cleanPath, "down")

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		downCmd := exec.CommandContext(cleanupCtx, cmdPrefix[0], downArgs...)
		_ = downCmd.Run()
	}

	if err := WaitForReady(ctx, m.targetURL, m.timeout); err != nil {
		teardown()
		return nil, fmt.Errorf("docker compose server readiness: %w", err)
	}

	return teardown, nil
}

// resolveDockerComposeCmd discovers whether `docker compose` or `docker-compose` is available.
func (m *LifecycleManager) resolveDockerComposeCmd() ([]string, error) {
	if len(m.dockerCmd) > 0 {
		return m.dockerCmd, nil
	}

	// 1. Try modern Docker CLI v2 plugin: `docker compose version`
	if dockerPath, err := exec.LookPath("docker"); err == nil {
		checkCmd := exec.Command(dockerPath, "compose", "version")
		if err := checkCmd.Run(); err == nil {
			return []string{dockerPath, "compose"}, nil
		}
	}

	// 2. Fallback to standalone legacy `docker-compose` v1
	if composePath, err := exec.LookPath("docker-compose"); err == nil {
		return []string{composePath}, nil
	}

	return nil, errors.New("neither 'docker compose' nor 'docker-compose' command was found on system PATH")
}
