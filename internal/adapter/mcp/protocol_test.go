package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   any             `json:"error,omitempty"`
}

func TestMCP_StdioProtocol_HandshakeAndTools(t *testing.T) {
	// 1. Build test binary
	tmpDir := t.TempDir()
	binName := "spectest"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tmpDir, binName)
	buildCmd := exec.Command("go", "build", "-o", binPath, "github.com/nguyentin05/spectest/cmd/spectest")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v, out: %s", err, string(out))
	}

	workspaceDir := filepath.Join(tmpDir, "workspace")
	_ = os.MkdirAll(filepath.Join(workspaceDir, "tests"), 0o755)

	// Write basic suite
	suiteContent := `version: 1
input: openapi
servers:
  - auth
apis:
  - GET {auth}/health
scenarios:
  - id: health-check
    name: Check health
    steps:
      - call: GET {auth}/health
        expect: 200
`
	_ = os.WriteFile(filepath.Join(workspaceDir, "tests", "spectest.yaml"), []byte(suiteContent), 0o600)

	// 2. Spawn `spectest mcp serve --root <workspace>`
	cmd := exec.Command(binPath, "mcp", "serve", "--root", workspaceDir)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start cmd: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	reader := bufio.NewReader(stdout)

	sendMsg := func(req jsonRPCRequest) {
		bytes, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("marshal rpc: %v", err)
		}
		_, _ = stdin.Write(append(bytes, '\n'))
	}

	readMsg := func() jsonRPCResponse {
		lineCh := make(chan []byte, 1)
		errCh := make(chan error, 1)

		go func() {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				errCh <- err
				return
			}
			lineCh <- line
		}()

		select {
		case line := <-lineCh:
			var resp jsonRPCResponse
			if err := json.Unmarshal(line, &resp); err != nil {
				t.Fatalf("STDOUT POLLUTION DETECTED! Expected JSON-RPC, got raw text: %s", string(line))
			}
			return resp
		case err := <-errCh:
			t.Fatalf("read from stdout: %v", err)
			return jsonRPCResponse{}
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for JSON-RPC response on stdout")
			return jsonRPCResponse{}
		}
	}

	// 3. Step 1: Handshake initialize
	sendMsg(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "integration-test-client",
				"version": "1.0",
			},
		},
	})

	initResp := readMsg()
	if fmt.Sprintf("%v", initResp.ID) != "1" {
		t.Errorf("expected id 1, got %v", initResp.ID)
	}
	if !strings.Contains(string(initResp.Result), "spectest") {
		t.Errorf("server name spectest not in initialize result: %s", string(initResp.Result))
	}

	// 4. Step 2: notifications/initialized
	sendMsg(jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	})

	// 5. Step 3: tools/list
	sendMsg(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	})

	listResp := readMsg()
	if fmt.Sprintf("%v", listResp.ID) != "2" {
		t.Errorf("expected id 2, got %v", listResp.ID)
	}
	resultStr := string(listResp.Result)
	expectedTools := []string{
		"spectest_status",
		"inspect_suite",
		"validate_suite",
		"plan_update",
		"start_update",
		"start_run",
		"get_job",
		"cancel_job",
		"read_report",
	}
	for _, tool := range expectedTools {
		if !strings.Contains(resultStr, tool) {
			t.Errorf("expected tool %q not found in tools/list response: %s", tool, resultStr)
		}
	}

	// 6. Step 4: tools/call inspect_suite
	sendMsg(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: map[string]any{
			"name": "inspect_suite",
			"arguments": map[string]any{
				"suite_path": "tests/spectest.yaml",
			},
		},
	})

	callResp := readMsg()
	if fmt.Sprintf("%v", callResp.ID) != "3" {
		t.Errorf("expected id 3, got %v", callResp.ID)
	}
	if !strings.Contains(string(callResp.Result), "health-check") {
		t.Errorf("inspect_suite did not return health-check scenario: %s", string(callResp.Result))
	}

	// Close stdin to gracefully terminate process
	_ = stdin.Close()
	_, _ = io.ReadAll(stdout)
}
