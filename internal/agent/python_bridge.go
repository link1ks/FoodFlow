package agent

import (
	"bufio"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

//go:embed python/runtime.py
var pythonRuntime string

func RuntimeDigest() string {
	sum := sha256.Sum256([]byte(pythonRuntime))
	return hex.EncodeToString(sum[:])
}

// Observation is only used by explicitly authorized offline/live evaluation tools.
// Application requests never enable it; provider output must not enter logs.
type Observation func(int, []byte)

type pythonRequest struct {
	Version   int        `json:"version"`
	Task      string     `json:"task"`
	Endpoint  string     `json:"endpoint,omitempty"`
	Model     string     `json:"model,omitempty"`
	Key       string     `json:"key,omitempty"`
	Messages  []*Message `json:"messages,omitempty"`
	Context   any        `json:"context,omitempty"`
	RecipeID  string     `json:"recipe_id,omitempty"`
	Data      []byte     `json:"data,omitempty"`
	MIME      string     `json:"mime,omitempty"`
	MaxTokens int        `json:"max_tokens,omitempty"`
	Observe   bool       `json:"observe,omitempty"`
}

type pythonFrame struct {
	Version          int    `json:"version"`
	Phase            string `json:"phase"`
	RequestBytes     int    `json:"request_bytes"`
	Content          string `json:"content"`
	Error            string `json:"error"`
	KnownComplete    bool   `json:"known_complete"`
	Status           int    `json:"status"`
	ProviderResponse string `json:"provider_response"`
}

func pythonExecutable() string {
	if value := os.Getenv("FOODFLOW_PYTHON"); value != "" {
		return value
	}
	// Native development prefers the user-created project environment.
	dir, _ := os.Getwd()
	for dir != "" {
		suffix := filepath.Join("bin", "python")
		if runtime.GOOS == "windows" {
			suffix = filepath.Join("Scripts", "python.exe")
		}
		candidate := filepath.Join(dir, ".cache", "python-agent-venv", suffix)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}

func pythonEnvironment() []string {
	environment := []string{}
	for _, key := range []string{"SYSTEMROOT", "WINDIR", "TEMP", "TMP", "TMPDIR", "LANG", "TZ"} {
		if value := os.Getenv(key); value != "" {
			environment = append(environment, key+"="+value)
		}
	}
	return environment
}

func callPython(ctx context.Context, request pythonRequest, guard CallGuard, observe Observation) (string, error) {
	timeout := 30 * time.Second
	if request.Task == "speech" {
		timeout = 65 * time.Second
	}
	if request.Task == "vision" {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request.Version = 1
	request.Observe = observe != nil
	cmd := exec.CommandContext(ctx, pythonExecutable(), "-I", "-u", "-c", pythonRuntime)
	cmd.Env = pythonEnvironment()
	cmd.Stderr = io.Discard
	input, err := cmd.StdinPipe()
	if err != nil {
		return "", errors.New("Python agent input unavailable")
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		return "", errors.New("Python agent output unavailable")
	}
	if err := cmd.Start(); err != nil {
		input.Close()
		output.Close()
		return "", errors.New("Python agent unavailable; check project interpreter")
	}
	defer func() { input.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), 512<<10)
	read := func() (pythonFrame, error) {
		var frame pythonFrame
		if !scanner.Scan() || json.Unmarshal(scanner.Bytes(), &frame) != nil || frame.Version != 1 {
			return frame, errors.New("Python agent protocol failed")
		}
		return frame, nil
	}
	if err := json.NewEncoder(input).Encode(request); err != nil {
		return "", errors.New("Python agent request failed")
	}
	frame, err := read()
	if err != nil {
		return "", err
	}
	demo := request.Task == "demo" || request.Task == "demo_advice"
	known := false
	if !demo {
		if frame.Phase != "prepared" || frame.RequestBytes <= 0 || frame.RequestBytes > 7<<20 {
			return "", errors.New("Python agent preparation failed")
		}
		// Application callers supply the durable guard. Diagnostic callers may
		// explicitly call text/vision adapters without it, as before migration.
		if guard != nil {
			finish, err := guard(ctx, frame.RequestBytes)
			if err != nil {
				return "", err
			}
			if finish == nil {
				return "", errors.New("invalid allowance settlement")
			}
			defer func() { finish(known) }()
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err := json.NewEncoder(input).Encode(map[string]any{"version": 1, "allow": true}); err != nil {
			return "", errors.New("Python agent authorization failed")
		}
		frame, err = read()
		if err != nil {
			return "", err
		}
	}
	if frame.Phase != "result" {
		return "", errors.New("Python agent result missing")
	}
	known = frame.KnownComplete
	if observe != nil {
		observe(frame.Status, []byte(frame.ProviderResponse))
	}
	if frame.Error != "" {
		if request.Task == "speech" {
			if frame.Status != 0 && frame.Status != 200 {
				return "", fmt.Errorf("语音服务返回状态 %d", frame.Status)
			}
			if frame.Status == 200 {
				return "", errors.New("未识别到有效内容，请重新录音")
			}
			return "", errors.New("语音服务连接失败；结果不确定，不会自动重发")
		}
		if request.Task == "vision" && frame.Status == 200 {
			return "", errors.New("未识别到明确的单种食材；请拍摄清晰的真实食材，避免杂物、包装文字或混合菜品")
		}
		return "", errors.New("Python agent rejected or could not complete the request")
	}
	return frame.Content, nil
}
