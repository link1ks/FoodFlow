package agent

import "testing"

func TestAdviceSchema(t *testing.T) {
	allowed := map[string]bool{"known": true}
	if _, err := ParseAdvice(`{"summary":"建议","tips":["搭配蔬菜"],"recipe_ids":["known"]}`, allowed); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{`{"summary":"建议","tips":["搭配"],"recipe_ids":["fake"]}`, `{"summary":"建议","tips":["搭配"],"recipe_ids":["known","known"]}`, `{"summary":"建议","tips":[],"recipe_ids":[]}`, `{"summary":"建议","tips":["搭配"],"recipe_ids":[],"inventory":999}`, `{"summary":"建议","tips":["搭配"],"recipe_ids":[]} {}`} {
		if _, err := ParseAdvice(s, allowed); err == nil {
			t.Fatal(s)
		}
	}
}
