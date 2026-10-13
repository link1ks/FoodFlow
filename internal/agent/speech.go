package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const MaxSpeechBytes = 5 << 20

// SpeechModel calls the Qwen3-ASR-Flash compatible endpoint once. It owns no
// database or inventory tools; its output is untrusted transcription text.
type SpeechModel struct {
	Endpoint, Name, Key string
	Client              *http.Client
	Guard               CallGuard
}

func SpeechFromEnv() SpeechModel {
	return SpeechModel{Endpoint: os.Getenv("SPEECH_MODEL_ENDPOINT"), Name: os.Getenv("SPEECH_MODEL_NAME"), Key: os.Getenv("SPEECH_MODEL_API_KEY")}
}

func (m SpeechModel) Configured() bool { return m.Endpoint != "" && m.Name != "" && m.Key != "" }

func (m SpeechModel) Transcribe(ctx context.Context, data []byte, mime string) (string, error) {
	if !m.Configured() || m.Guard == nil {
		return "", errors.New("语音模型或调用保护未配置")
	}
	if len(data) == 0 || len(data) > MaxSpeechBytes {
		return "", errors.New("录音大小无效")
	}
	switch mime {
	case "audio/wav", "audio/webm", "audio/ogg", "audio/mpeg":
	default:
		return "", errors.New("不支持的录音格式")
	}
	payload := map[string]any{
		"model": m.Name, "stream": false,
		"messages":    []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_audio", "input_audio": map[string]string{"data": "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)}}}}},
		"asr_options": map[string]any{"language": "zh", "enable_itn": false},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", errors.New("无法准备录音")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(m.Endpoint, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", errors.New("语音服务地址无效")
	}
	req.Header.Set("Authorization", "Bearer "+m.Key)
	req.Header.Set("Content-Type", "application/json")
	finish, err := m.Guard(ctx, len(body))
	if err != nil {
		return "", err
	}
	complete := false
	defer func() { finish(complete) }()
	client := m.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := noRedirectClient(client).Do(req)
	if err != nil {
		return "", errors.New("语音服务连接失败；结果不确定，不会自动重发")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		complete = true
		return "", fmt.Errorf("语音服务返回状态 %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if err != nil {
		return "", errors.New("语音结果读取失败；不会自动重发")
	}
	complete = true
	if len(raw) > 65536 {
		return "", errors.New("语音结果过长")
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(raw, &response) != nil || len(response.Choices) != 1 {
		return "", errors.New("语音结果格式无效")
	}
	text := strings.TrimSpace(response.Choices[0].Message.Content)
	if text == "" || len([]rune(text)) > 2000 {
		return "", errors.New("未识别到有效内容，请重新录音")
	}
	return text, nil
}
