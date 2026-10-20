package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func SystemMessage(content string) *Message { return &Message{Role: "system", Content: content} }
func UserMessage(content string) *Message   { return &Message{Role: "user", Content: content} }

type ChatModel interface {
	Generate(context.Context, []*Message) (*Message, error)
}
type CallGuard func(context.Context, int) (func(bool), error)
type DemoModel struct{ RecipeID string }

func (d DemoModel) Generate(ctx context.Context, _ []*Message) (*Message, error) {
	content, err := callPython(ctx, pythonRequest{Task: "demo", RecipeID: d.RecipeID}, nil, nil)
	if err != nil {
		return nil, err
	}
	return &Message{Role: "assistant", Content: content}, nil
}

type OpenAIModel struct {
	Endpoint, Name, Key string
	MaxTokens           int
	Guard               CallGuard
	Task                string
	Context             any
	Observe             Observation
}

func (m OpenAIModel) Generate(ctx context.Context, messages []*Message) (*Message, error) {
	task := m.Task
	if task == "" {
		task = "text"
	}
	content, err := callPython(ctx, pythonRequest{Task: task, Endpoint: m.Endpoint, Model: m.Name, Key: m.Key, Messages: messages, MaxTokens: m.MaxTokens, Context: m.Context}, m.Guard, m.Observe)
	if err != nil {
		return nil, err
	}
	return &Message{Role: "assistant", Content: content}, nil
}
func DemoAdvice(ctx context.Context, context any, allowed map[string]bool) (Advice, error) {
	content, err := callPython(ctx, pythonRequest{Task: "demo_advice", Context: context}, nil, nil)
	if err != nil {
		return Advice{}, err
	}
	return ParseAdvice(content, allowed)
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
