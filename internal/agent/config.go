package agent

import (
	"net/url"
	"os"
)

func applyProviderOptions(payload map[string]any, endpoint string) {
	u, err := url.Parse(endpoint)
	if err == nil && u.Hostname() == "api.deepseek.com" {
		payload["thinking"] = map[string]string{"type": "disabled"}
	}
}

// VisionFromEnv uses a separate credential pair when either override is set.
// Never send the text service's key to an independently configured endpoint.
func VisionFromEnv() VisionModel {
	endpoint, key := os.Getenv("VISION_MODEL_ENDPOINT"), os.Getenv("VISION_MODEL_API_KEY")
	if endpoint == "" && key == "" {
		endpoint, key = os.Getenv("MODEL_ENDPOINT"), os.Getenv("MODEL_API_KEY")
	}
	return VisionModel{Endpoint: endpoint, Key: key, Name: os.Getenv("VISION_MODEL_NAME")}
}

func (m VisionModel) Configured() bool {
	return m.Endpoint != "" && m.Key != "" && m.Name != ""
}

// Conservative retry protection also covers partially configured services.
func ExternalModelConfigured() bool {
	return os.Getenv("MODEL_ENDPOINT") != "" || os.Getenv("MODEL_API_KEY") != "" ||
		os.Getenv("VISION_MODEL_ENDPOINT") != "" || os.Getenv("VISION_MODEL_API_KEY") != ""
}
