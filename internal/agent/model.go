package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"io"
	"net/http"
	"strings"
	"time"
)

type DemoModel struct{ RecipeID string }

var _ model.BaseChatModel = DemoModel{}

func (d DemoModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	raw, _ := json.Marshal(struct {
		RecipeID string `json:"recipe_id"`
	}{d.RecipeID})
	return schema.AssistantMessage(string(raw), nil), nil
}
func (d DemoModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("streaming not used by FoodFlow planner")
}

type OpenAIModel struct {
	Endpoint, Name, Key string
	Client              *http.Client
	MaxTokens           int
	Guard               CallGuard
}

// The application provides a durable allowance guard; adapters never own DB access.
// knownComplete=false retains an uncertain debit/slot after network ambiguity.
type CallGuard func(context.Context, int) (func(bool), error)

func noRedirectClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{Timeout: 25 * time.Second}
	}
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &copy
}

var _ model.BaseChatModel = OpenAIModel{}

func (m OpenAIModel) Generate(ctx context.Context, messages []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	if m.Endpoint == "" || m.Name == "" || m.Key == "" {
		return nil, errors.New("model configuration incomplete")
	}
	payload := map[string]any{"model": m.Name, "temperature": 0, "response_format": map[string]string{"type": "json_object"}, "messages": messages}
	payload["max_tokens"] = 1600
	applyProviderOptions(payload, m.Endpoint)
	if m.MaxTokens > 0 {
		payload["max_tokens"] = m.MaxTokens
	}
	body, _ := json.Marshal(payload)
	req, e := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(m.Endpoint, "/")+"/chat/completions", bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Authorization", "Bearer "+m.Key)
	req.Header.Set("Content-Type", "application/json")
	knownComplete := false
	if m.Guard != nil {
		finish, err := m.Guard(ctx, len(body))
		if err != nil {
			return nil, err
		}
		defer func() { finish(knownComplete) }()
	}
	client := noRedirectClient(m.Client)
	resp, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		knownComplete = true
		return nil, fmt.Errorf("model status %d", resp.StatusCode)
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if e != nil {
		return nil, e
	}
	if len(raw) > 65536 {
		return nil, errors.New("model response too large")
	}
	knownComplete = true
	var envelope struct {
		Choices []struct {
			Message schema.Message `json:"message"`
		} `json:"choices"`
	}
	if e = json.Unmarshal(raw, &envelope); e != nil || len(envelope.Choices) != 1 {
		return nil, errors.New("invalid model response")
	}
	return &envelope.Choices[0].Message, nil
}
func (m OpenAIModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("streaming not used by FoodFlow planner")
}
func ParseRecipeID(content string) (string, error) {
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.DisallowUnknownFields()
	var out struct {
		RecipeID string `json:"recipe_id"`
	}
	if e := decoder.Decode(&out); e != nil {
		return "", fmt.Errorf("invalid structured output: %w", e)
	}
	if out.RecipeID == "" {
		return "", errors.New("recipe_id required")
	}
	var extra any
	if e := decoder.Decode(&extra); e != io.EOF {
		return "", errors.New("multiple JSON values")
	}
	return out.RecipeID, nil
}
