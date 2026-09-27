package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type Advice struct {
	Summary   string   `json:"summary"`
	Tips      []string `json:"tips"`
	RecipeIDs []string `json:"recipe_ids"`
}

func ParseAdvice(raw string, allowed map[string]bool) (Advice, error) {
	var out Advice
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return out, fmt.Errorf("建议格式不符合约定: %w", err)
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return out, fmt.Errorf("建议包含多段数据")
	}
	if strings.TrimSpace(out.Summary) == "" || len([]rune(out.Summary)) > 1000 || len(out.Tips) < 1 || len(out.Tips) > 6 || out.RecipeIDs == nil || len(out.RecipeIDs) > 3 {
		return out, fmt.Errorf("建议长度或字段无效")
	}
	for _, s := range out.Tips {
		if strings.TrimSpace(s) == "" || len([]rune(s)) > 500 {
			return out, fmt.Errorf("建议条目无效")
		}
	}
	seen := map[string]bool{}
	for _, id := range out.RecipeIDs {
		if !allowed[id] || seen[id] {
			return out, fmt.Errorf("模型选择了未知或重复菜谱")
		}
		seen[id] = true
	}
	return out, nil
}
