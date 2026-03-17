package main_test

import (
	"bufio"
	"bytes"
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

// binaryPath holds the path to the compiled ricket binary, built once in TestMain.
var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary into a temp dir so all subtests share one compile.
	tmp, err := os.MkdirTemp("", "ricket-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)

	name := "ricket"
	if runtime.GOOS == "windows" {
		name = "ricket.exe"
	}
	binaryPath = filepath.Join(tmp, name)

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "go build failed: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// testVaultPath returns the absolute path to the testdata/vault fixture.
func testVaultPath(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "testdata", "vault"))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// runRicket runs the ricket binary with the given args and returns stdout, stderr, exit code.
func runRicket(t *testing.T, env []string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// ── CLI tests ─────────────────────────────────────────────────────────────────

func TestCLI_Status(t *testing.T) {
	vault := testVaultPath(t)
	stdout, _, code := runRicket(t, nil,
		"status", "--vault-root", vault)

	if code != 0 {
		t.Fatalf("ricket status exited %d\nstdout: %s", code, stdout)
	}
	if !strings.Contains(stdout, "Total notes:") {
		t.Errorf("expected 'Total notes:' in output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Inbox:") {
		t.Errorf("expected 'Inbox:' in output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Categories:") {
		t.Errorf("expected 'Categories:' in output, got:\n%s", stdout)
	}
	// Vault has 3 inbox notes
	if !strings.Contains(stdout, "Inbox:       3 notes") {
		t.Errorf("expected 3 inbox notes, got:\n%s", stdout)
	}
}

func TestCLI_Status_EnvVar(t *testing.T) {
	vault := testVaultPath(t)
	stdout, _, code := runRicket(t,
		[]string{"RICKET_VAULT_ROOT=" + vault},
		"status")

	if code != 0 {
		t.Fatalf("exited %d: %s", code, stdout)
	}
	if !strings.Contains(stdout, "Total notes:") {
		t.Errorf("expected 'Total notes:' in output")
	}
}

func TestCLI_ConfigPath(t *testing.T) {
	vault := testVaultPath(t)
	stdout, _, code := runRicket(t, nil,
		"config", "path", "--vault-root", vault)

	if code != 0 {
		t.Fatalf("ricket config path exited %d\nstdout: %s", code, stdout)
	}
	// Output should be the vault path (trimmed)
	got := strings.TrimSpace(stdout)
	if got != vault {
		t.Errorf("config path = %q, want %q", got, vault)
	}
}

func TestCLI_Status_MissingVault(t *testing.T) {
	_, _, code := runRicket(t, nil,
		"status", "--vault-root", "/nonexistent/path/that/does/not/exist")
	if code == 0 {
		t.Error("expected non-zero exit code for missing vault")
	}
}

func TestCLI_Version(t *testing.T) {
	stdout, _, code := runRicket(t, nil, "--version")
	if code != 0 {
		t.Fatalf("--version exited %d", code)
	}
	if !strings.Contains(stdout, "0.1.0") {
		t.Errorf("expected version 0.1.0 in output: %s", stdout)
	}
}

func TestCLI_ConfigValidate(t *testing.T) {
	vault := testVaultPath(t)
	stdout, stderr, code := runRicket(t, nil,
		"config", "validate", "--vault-root", vault)

	if code != 0 {
		t.Fatalf("ricket config validate exited %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Vault configuration looks good.") {
		t.Errorf("expected success message, got stdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if !strings.Contains(stdout, "inbox directory exists") {
		t.Errorf("expected inbox OK in output: %s", stdout)
	}
}

func TestCLI_Init_ExistingConfig(t *testing.T) {
	vault := testVaultPath(t)
	// testdata/vault already has ricket.yaml — init should fail
	_, stderr, code := runRicket(t, nil, "init", vault)
	if code == 0 {
		t.Error("expected non-zero exit when ricket.yaml already exists")
	}
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("expected 'already exists' in stderr: %s", stderr)
	}
}

// ── MCP JSON-RPC E2E test ─────────────────────────────────────────────────────

// copyDirAll recursively copies src to dst (dst must already be a temp dir).
func copyDirAll(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

// TestMCP_E2E spawns `ricket serve` as a subprocess and drives it over
// stdin/stdout using the MCP JSON-RPC 2.0 protocol.
func TestMCP_E2E(t *testing.T) {
	// Use a temp copy of the vault so mutation calls don't modify the fixture.
	tmpVault := t.TempDir()
	if err := copyDirAll(testVaultPath(t), tmpVault); err != nil {
		t.Fatalf("copy vault: %v", err)
	}
	vault := tmpVault

	cmd := exec.Command(binaryPath, "serve", "--vault-root", vault)
	cmd.Env = os.Environ() // inherit env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stdin.Close()
		cmd.Process.Kill() //nolint:errcheck
		cmd.Wait()         //nolint:errcheck
	})

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1 MiB max line

	send := func(msg map[string]any) {
		t.Helper()
		data, _ := json.Marshal(msg)
		data = append(data, '\n')
		if _, err := io.WriteString(stdin, string(data)); err != nil {
			t.Fatalf("write to server stdin: %v", err)
		}
	}

	// readResponse reads the next JSON object from the server with a timeout.
	readResponse := func() map[string]any {
		t.Helper()
		done := make(chan map[string]any, 1)
		go func() {
			if scanner.Scan() {
				var m map[string]any
				json.Unmarshal(scanner.Bytes(), &m) //nolint:errcheck
				done <- m
			} else {
				done <- nil
			}
		}()
		select {
		case m := <-done:
			if m == nil {
				t.Fatal("server closed stdout before sending response")
			}
			return m
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for MCP server response")
			return nil
		}
	}

	// ── 1. initialize ───────────────────────────────────────────────────
	send(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "0.0.1"},
		},
	})
	initResp := readResponse()
	if initResp["error"] != nil {
		t.Fatalf("initialize error: %v", initResp["error"])
	}
	result, ok := initResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("initialize result is not object: %T", initResp["result"])
	}
	if result["protocolVersion"] == nil {
		t.Error("initialize response missing protocolVersion")
	}

	// ── 2. initialized notification (no response expected) ───────────────
	send(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})

	// ── 3. tools/list ───────────────────────────────────────────────────
	send(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]any{},
	})
	listResp := readResponse()
	if listResp["error"] != nil {
		t.Fatalf("tools/list error: %v", listResp["error"])
	}
	listResult, ok := listResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list result: %T", listResp["result"])
	}
	tools, ok := listResult["tools"].([]any)
	if !ok {
		t.Fatalf("tools/list result.tools: %T", listResult["tools"])
	}
	if len(tools) != 9 {
		t.Errorf("expected 9 tools, got %d", len(tools))
	}
	toolNames := make([]string, 0, len(tools))
	for _, raw := range tools {
		tool := raw.(map[string]any)
		toolNames = append(toolNames, tool["name"].(string))
	}
	for _, want := range []string{
		"vault_list_inbox", "vault_read_note", "vault_search",
		"vault_get_categories", "vault_get_templates",
		"vault_file_note", "vault_create_note", "vault_update_note", "vault_status",
	} {
		found := false
		for _, n := range toolNames {
			if n == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("tool %q not in list: %v", want, toolNames)
		}
	}

	// ── 4. tools/call vault_status ──────────────────────────────────────
	send(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "vault_status",
			"arguments": map[string]any{},
		},
	})
	statusResp := readResponse()
	if statusResp["error"] != nil {
		t.Fatalf("vault_status call error: %v", statusResp["error"])
	}
	callResult, ok := statusResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("vault_status result: %T", statusResp["result"])
	}
	content, ok := callResult["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("vault_status content: %T %v", callResult["content"], callResult["content"])
	}
	first := content[0].(map[string]any)
	text, _ := first["text"].(string)
	var statusData map[string]any
	if err := json.Unmarshal([]byte(text), &statusData); err != nil {
		t.Fatalf("unmarshal vault_status response: %v\ntext: %s", err, text)
	}
	inboxCount, _ := statusData["inboxCount"].(float64)
	if int(inboxCount) != 3 {
		t.Errorf("inboxCount = %d, want 3", int(inboxCount))
	}

	// ── 5. tools/call vault_list_inbox ──────────────────────────────────
	send(map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "vault_list_inbox",
			"arguments": map[string]any{},
		},
	})
	inboxResp := readResponse()
	if inboxResp["error"] != nil {
		t.Fatalf("vault_list_inbox call error: %v", inboxResp["error"])
	}
	inboxCallResult, _ := inboxResp["result"].(map[string]any)
	inboxContent, _ := inboxCallResult["content"].([]any)
	if len(inboxContent) == 0 {
		t.Fatal("vault_list_inbox returned empty content")
	}
	inboxText, _ := inboxContent[0].(map[string]any)["text"].(string)
	var inboxItems []any
	if err := json.Unmarshal([]byte(inboxText), &inboxItems); err != nil {
		t.Fatalf("unmarshal inbox list: %v\ntext: %s", err, inboxText)
	}
	if len(inboxItems) != 3 {
		t.Errorf("inbox items = %d, want 3", len(inboxItems))
	}

	// ── 6. tools/call vault_update_note (adds a tag to an existing note) ──
	// Note: testdata/vault is read-only fixture; this call will succeed at the
	// MCP protocol level even though gitCommitted may be false (no git repo in temp copy).
	send(map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "vault_update_note",
			"arguments": map[string]any{
				"path": "Areas/Engineering/decisions/use-sqlite-for-index.md",
				"tags": []any{"reviewed"},
			},
		},
	})
	updateResp := readResponse()
	if updateResp["error"] != nil {
		t.Fatalf("vault_update_note call error: %v", updateResp["error"])
	}
	updateCallResult, _ := updateResp["result"].(map[string]any)
	updateContent, _ := updateCallResult["content"].([]any)
	if len(updateContent) == 0 {
		t.Fatal("vault_update_note returned empty content")
	}
	updateText, _ := updateContent[0].(map[string]any)["text"].(string)
	var updateData map[string]any
	if err := json.Unmarshal([]byte(updateText), &updateData); err != nil {
		t.Fatalf("unmarshal update response: %v", err)
	}
	if updateData["path"] != "Areas/Engineering/decisions/use-sqlite-for-index.md" {
		t.Errorf("update path = %q", updateData["path"])
	}

	// ── 7. tools/call vault_read_note ────────────────────────────────────
	send(map[string]any{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "vault_read_note",
			"arguments": map[string]any{
				"path": "Areas/Engineering/decisions/use-sqlite-for-index.md",
			},
		},
	})
	readResp := readResponse()
	if readResp["error"] != nil {
		t.Fatalf("vault_read_note call error: %v", readResp["error"])
	}
	readCallResult, _ := readResp["result"].(map[string]any)
	readContent, _ := readCallResult["content"].([]any)
	if len(readContent) == 0 {
		t.Fatal("vault_read_note returned empty content")
	}
	readText, _ := readContent[0].(map[string]any)["text"].(string)
	var readNote map[string]any
	if err := json.Unmarshal([]byte(readText), &readNote); err != nil {
		t.Fatalf("unmarshal read_note: %v", err)
	}
	if readNote["name"] != "use-sqlite-for-index" {
		t.Errorf("note name = %q, want 'use-sqlite-for-index'", readNote["name"])
	}
}
