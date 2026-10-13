package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQwenSpeechRequestAndGuard(t *testing.T) {
	calls := 0
	finished := false
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer speech-only" {
			t.Error("wrong speech endpoint/key")
		}
		var body struct {
			Model    string
			Stream   bool
			Messages []struct {
				Content []struct {
					Type       string
					InputAudio struct{ Data string } `json:"input_audio"`
				}
			}
			Options struct {
				Language string
				ITN      bool `json:"enable_itn"`
			} `json:"asr_options"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || body.Model != "qwen3-asr-flash" || body.Stream || body.Options.Language != "zh" || body.Options.ITN || len(body.Messages) != 1 || len(body.Messages[0].Content) != 1 || body.Messages[0].Content[0].Type != "input_audio" || body.Messages[0].Content[0].InputAudio.Data != "data:audio/wav;base64,AQID" {
			t.Error("wrong Qwen ASR payload")
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"西红柿半斤"}}]}`))
	}))
	defer s.Close()
	m := SpeechModel{Endpoint: s.URL + "/v1", Name: "qwen3-asr-flash", Key: "speech-only", Guard: func(_ context.Context, n int) (func(bool), error) {
		if n < 3 {
			t.Error("missing request size")
		}
		return func(known bool) { finished = known }, nil
	}}
	got, err := m.Transcribe(context.Background(), []byte{1, 2, 3}, "audio/wav")
	if err != nil || got != "西红柿半斤" || calls != 1 || !finished {
		t.Fatal(got, err, calls, finished)
	}
	m.Guard = func(context.Context, int) (func(bool), error) { return nil, errors.New("blocked") }
	if _, err = m.Transcribe(context.Background(), []byte{1}, "audio/wav"); err == nil || calls != 1 {
		t.Fatal("guard did not block dispatch")
	}
	t.Setenv("MODEL_API_KEY", "text-key")
	t.Setenv("MODEL_ENDPOINT", s.URL)
	t.Setenv("SPEECH_MODEL_API_KEY", "")
	t.Setenv("SPEECH_MODEL_ENDPOINT", "")
	t.Setenv("SPEECH_MODEL_NAME", "")
	if SpeechFromEnv().Configured() {
		t.Fatal("speech inherited text configuration")
	}
}

func TestSpeechFailureDoesNotResendOrExposePayload(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"redirect", 307, "private"}, {"provider", 500, "private transcript"}, {"empty", 200, `{"choices":[]}`}, {"oversized", 200, strings.Repeat("private", 10000)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			known := false
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Location", "/again")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer s.Close()
			m := SpeechModel{Endpoint: s.URL, Name: "qwen3-asr-flash", Key: "secret", Guard: func(context.Context, int) (func(bool), error) { return func(v bool) { known = v }, nil }}
			_, err := m.Transcribe(context.Background(), []byte{1}, "audio/wav")
			if err == nil || calls != 1 || !known || strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "secret") {
				t.Fatal(err, calls, known)
			}
		})
	}
}
