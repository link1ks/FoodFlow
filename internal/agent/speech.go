package agent

import (
	"context"
	"errors"
	"os"
)

const MaxSpeechBytes = 5 << 20

type SpeechModel struct {
	Endpoint, Name, Key string
	Guard               CallGuard
	Observe             Observation
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
	return callPython(ctx, pythonRequest{Task: "speech", Endpoint: m.Endpoint, Model: m.Name, Key: m.Key, Data: data, MIME: mime}, m.Guard, m.Observe)
}
