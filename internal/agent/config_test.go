package agent

import "testing"

func TestVisionConfig(t *testing.T) {
	t.Setenv("MODEL_ENDPOINT", "https://text.invalid/v1")
	t.Setenv("MODEL_API_KEY", "text-secret")
	t.Setenv("VISION_MODEL_NAME", "vision")
	t.Setenv("VISION_MODEL_ENDPOINT", "")
	t.Setenv("VISION_MODEL_API_KEY", "")
	if m := VisionFromEnv(); !m.Configured() || m.Key != "text-secret" {
		t.Fatal("shared configuration unavailable")
	}
	t.Setenv("VISION_MODEL_ENDPOINT", "https://vision.invalid/v1")
	if m := VisionFromEnv(); m.Configured() || m.Key != "" {
		t.Fatal("must not forward text credentials to independent endpoint")
	}
	t.Setenv("VISION_MODEL_API_KEY", "vision-secret")
	if m := VisionFromEnv(); !m.Configured() || m.Key != "vision-secret" || !ExternalModelConfigured() {
		t.Fatal("independent configuration unavailable")
	}
}
