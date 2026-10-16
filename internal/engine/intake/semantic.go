package intake

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var utteranceBreak = regexp.MustCompile(`[，,、；;。\n]+`)
var correction = regexp.MustCompile(`不对|不是|错了|更正为|更正|纠正一下|改成|改为`)
var exclusion = regexp.MustCompile(`不要|不用|别加|别放|别买|不添加|不加|不放|没有买|没买|取消|去掉|去除|撤回|撤销|删掉|删除`)

func hasAny(text string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

// Interpret accepts a bounded ingredient-addition grammar. Every clause must be
// consumed; finding a food name somewhere in unrelated speech is insufficient.
// Corrections only change this utterance's drafts, never existing stock.
func Interpret(text string, catalog []CatalogItem) Result {
	out := Result{Decision: "noop", Transcript: text, Items: []Draft{}, Warning: "本入口只处理本次新增食材，请说出食材、数量和单位。"}
	if !utf8.ValidString(text) || len([]rune(text)) > 2000 || strings.TrimSpace(text) == "" {
		return out
	}
	// Reject the entire utterance: do not salvage numerical fragments from a
	// reminder, question, quotation, instruction or an existing-stock command.
	if hasAny(text, "明天", "后天", "下周", "提醒", "打算", "准备买", "想买", "查询", "多少", "营养", "热量", "做什么", "怎么做", "菜谱", "能做", "可以做", "举例", "例如", "昨天", "上次", "不新增", "不录入", "不要动", "结束录入", "系统", "规则", "提示词", "密钥", "命令", "数据库", "包装上", "写着", "文字", "http", "“", "”", "\"", "「", "」") {
		return out
	}
	clarify := func() Result {
		r := Parse(text, catalog)
		r.Decision = "clarify"
		r.Warning = "这段内容无法明确作为本次新增，请只说食材、数量和单位，或使用目录手动填写。"
		for i := range r.Items {
			r.Items[i].Quantity = ""
			r.Items[i].Warning = "请核对原文后填写。"
		}
		return r
	}
	if hasAny(text, "库存", "记录", "大概", "差不多", "左右", "一些", "一点点", "若干", "还是", "或者", "没决定", "单位", "每瓶", "每盒", "每袋", "每包", "一共", "总共") {
		return clarify()
	}
	// Aliases are replaced longest first. Non-catalogue nouns remain in the
	// residual clause and prevent it from producing an accepted draft.
	type nameEntry struct {
		name string
		item CatalogItem
	}
	names := []nameEntry{}
	for _, item := range catalog {
		for _, name := range append([]string{item.Name}, item.Aliases...) {
			if name != "" {
				names = append(names, nameEntry{name, item})
			}
		}
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i].name) > len(names[j].name) })
	mentions := func(clause string) []CatalogItem {
		found := []CatalogItem{}
		seen := map[string]bool{}
		for _, entry := range names {
			if strings.Contains(clause, entry.name) {
				if !seen[entry.item.ID] {
					found = append(found, entry.item)
					seen[entry.item.ID] = true
				}
				clause = strings.ReplaceAll(clause, entry.name, " ")
			}
		}
		return found
	}
	residual := func(clause string) string {
		for _, entry := range names {
			clause = strings.ReplaceAll(clause, entry.name, " ")
		}
		clause = amount.ReplaceAllString(clause, " ")
		clause = correction.ReplaceAllString(clause, " ")
		clause = exclusion.ReplaceAllString(clause, " ")
		for _, word := range []string{"请帮我", "帮我", "刚刚买了", "刚买了", "买了", "添加", "新增", "入库", "改买", "现在", "刚才的操作", "刚才的添加", "这一项", "全部", "只", "再", "加", "要", "了", "我", "是", "和", "与", "把", "不变", "呃", "嗯", "那个", "：", ":", " ", "\t", "\r"} {
			clause = strings.ReplaceAll(clause, word, "")
		}
		return clause
	}
	state := map[string]Draft{}
	order := []string{}
	mentioned := map[string]CatalogItem{}
	pendingCorrection := false
	lastName := ""
	for _, rawClause := range utteranceBreak.Split(text, -1) {
		clause := strings.TrimSpace(rawClause)
		if clause == "" {
			continue
		}
		items := mentions(clause)
		for _, item := range items {
			mentioned[item.ID] = item
		}
		if len(mentioned) > 20 {
			return clarify()
		}
		if hasAny(clause, "它", "那个不要", "前面", "后面", "这个") && len(mentioned) != 1 {
			return clarify()
		}
		isExclusion := exclusion.MatchString(clause)
		isCorrection := correction.MatchString(clause) || pendingCorrection
		if isExclusion && isCorrection {
			return clarify()
		}
		if residual(clause) != "" {
			return out
		}
		if isExclusion && !isCorrection {
			if len(items) == 0 {
				if strings.Contains(clause, "全部") {
					state = map[string]Draft{}
				} else if len(mentioned) == 1 {
					for id := range mentioned {
						delete(state, id)
					}
				} else {
					return clarify()
				}
			} else {
				for _, item := range items {
					delete(state, item.ID)
				}
			}
			continue
		}
		if strings.Contains(clause, "不变") {
			if len(items) != 1 || state[items[0].ID].CatalogID == "" {
				return clarify()
			}
			continue
		}
		if isCorrection {
			if len(items) > 1 {
				return clarify()
			}
			var target CatalogItem
			if len(items) == 1 {
				target = items[0]
			} else if len(mentioned) == 1 {
				for _, item := range mentioned {
					target = item
				}
			}
			parts := correction.Split(clause, -1)
			if target.ID != "" && state[target.ID].CatalogID == "" && len(parts) > 1 {
				initial := Parse(parts[0], catalog)
				if len(initial.Items) == 1 && initial.Items[0].CatalogID == target.ID && initial.Items[0].Quantity != "" {
					state[target.ID] = initial.Items[0]
					order = append(order, target.ID)
				}
			}
			final := strings.TrimSpace(parts[len(parts)-1])
			matches := amount.FindAllString(final, -1)
			if len(matches) == 0 {
				pendingCorrection = true
				continue
			}
			if target.ID == "" || len(matches) != 1 || state[target.ID].CatalogID == "" {
				return clarify()
			}
			parsed := Parse(target.Name+matches[0], []CatalogItem{target})
			if len(parsed.Items) != 1 || parsed.Items[0].Quantity == "" {
				return clarify()
			}
			parsed.Items[0].Heard = text
			state[target.ID] = parsed.Items[0]
			pendingCorrection = false
			continue
		}
		if len(items) == 0 {
			// Punctuation between a single name and its amount is common ASR
			// output. Never attach it across multiple possible ingredients.
			if amount.MatchString(clause) {
				if lastName == "" || len(mentioned) != 1 {
					return clarify()
				}
				clause = mentioned[lastName].Name + clause
			} else {
				continue
			}
		}
		clause = strings.NewReplacer("：", "", ":", "").Replace(clause)
		parsed := Parse(clause, catalog)
		if len(parsed.Items) == 0 {
			return clarify()
		}
		for _, item := range parsed.Items {
			if item.CatalogID == "" {
				return clarify()
			}
			if old, ok := state[item.CatalogID]; ok && old.Quantity != "" {
				return clarify()
			}
			if _, ok := state[item.CatalogID]; !ok {
				order = append(order, item.CatalogID)
			}
			item.Heard = text
			state[item.CatalogID] = item
			lastName = item.CatalogID
		}
	}
	if pendingCorrection {
		return clarify()
	}
	seen := map[string]bool{}
	for _, id := range order {
		if item, ok := state[id]; ok && !seen[id] {
			out.Items = append(out.Items, item)
			seen[id] = true
		}
	}
	if len(out.Items) == 0 {
		return out
	}
	out.Decision = "draft"
	out.Warning = "已按本次新增整理；请核对食材、最终数量和单位后确认。"
	for _, item := range out.Items {
		if item.Quantity == "" {
			out.Decision = "clarify"
		}
	}
	return out
}
