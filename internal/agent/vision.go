package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ImageSuggestion is deliberately limited to fields a model can plausibly see.
// Quantity, unit, purchase date and expiry must be supplied by a user.
type ImageSuggestion struct {
	Decision string `json:"decision"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type VisionModel struct {
	Endpoint, Name, Key string
	Guard               CallGuard
	Observe             Observation
}

func ParseImageSuggestion(content string) (ImageSuggestion, error) {
	dec := json.NewDecoder(strings.NewReader(content))
	dec.DisallowUnknownFields()
	var observation struct {
		Scene string   `json:"scene"`
		Foods []string `json:"foods"`
	}
	var v ImageSuggestion
	if e := dec.Decode(&observation); e != nil {
		return v, fmt.Errorf("invalid image result: %w", e)
	}
	if e := dec.Decode(new(any)); e != io.EOF {
		return v, errors.New("multiple image results")
	}
	if observation.Scene != "photo" || len(observation.Foods) != 1 {
		return ImageSuggestion{}, errors.New("未识别到明确的单种食材；请拍摄清晰的真实食材，避免杂物、包装文字或混合菜品")
	}
	v.Decision = "ingredient"
	v.Name = strings.TrimSpace(observation.Foods[0])
	if v.Name == "" || len([]rune(v.Name)) > 100 || len([]rune(v.Category)) > 60 {
		return v, errors.New("image result needs a valid ingredient name")
	}
	return v, nil
}

func (m VisionModel) Recognize(ctx context.Context, data []byte, mime string) (ImageSuggestion, error) {
	if len(data) == 0 || len(data) > 5<<20 {
		return ImageSuggestion{}, errors.New("invalid image size")
	}
	content, err := callPython(ctx, pythonRequest{Task: "vision", Endpoint: m.Endpoint, Model: m.Name, Key: m.Key, Data: data, MIME: mime}, m.Guard, m.Observe)
	if err != nil {
		return ImageSuggestion{}, err
	}
	return ParseImageSuggestion(content)
}
