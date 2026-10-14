// Package intake conservatively prepares editable catalogue drafts from speech
// text. It does not infer package sizes, expiry dates or authoritative stock.
package intake

import (
	"foodflow/internal/core"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type CatalogItem struct {
	ID, Name, Unit string
	Aliases        []string
}
type Draft struct {
	CatalogID string `json:"catalog_id"`
	Name      string `json:"name"`
	Unit      string `json:"unit"`
	Quantity  string `json:"quantity"`
	Heard     string `json:"heard"`
	Warning   string `json:"warning"`
}
type Result struct {
	Transcript string  `json:"transcript"`
	Items      []Draft `json:"items"`
	Warning    string  `json:"warning"`
}

// After 点, only digits belong to the fraction. Otherwise 一点五千克
// greedily consumes the unit's 千 and incorrectly parses 一点五千 + 克.
var amount = regexp.MustCompile(`([-+]?\d+(?:\.\d+)?|[零〇一二两三四五六七八九十百千万半]+(?:点[零〇一二两三四五六七八九]+)?)\s*(公斤|千克|毫升|kg|ml|克|斤|两|升|g|l|个|枚|只|包|袋|盒|瓶|根)`)
var separator = regexp.MustCompile(`[，,、；;。\n]`)

func Parse(text string, catalog []CatalogItem) Result {
	out := Result{Transcript: text, Items: []Draft{}, Warning: "请逐项核对；未提及的食材、数量、日期和位置不会自动补全。"}
	type alias struct {
		name string
		item CatalogItem
	}
	aliases := []alias{}
	owners := map[string]map[string]bool{}
	for _, item := range catalog {
		for _, name := range append([]string{item.Name}, item.Aliases...) {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if owners[name] == nil {
				owners[name] = map[string]bool{}
			}
			owners[name][item.ID] = true
			aliases = append(aliases, alias{name, item})
		}
	}
	sort.Slice(aliases, func(i, j int) bool {
		if len(aliases[i].name) != len(aliases[j].name) {
			return len(aliases[i].name) > len(aliases[j].name)
		}
		return aliases[i].item.ID < aliases[j].item.ID
	})
	for _, clause := range separator.Split(text, -1) {
		type hit struct {
			start, end int
			a          alias
		}
		hits := []hit{}
		for _, a := range aliases {
			for offset := 0; offset < len(clause); {
				at := strings.Index(clause[offset:], a.name)
				if at < 0 {
					break
				}
				at += offset
				end := at + len(a.name)
				offset = end
				overlap := false
				for _, h := range hits {
					if at < h.end && h.start < end {
						overlap = true
						break
					}
				}
				if !overlap {
					hits = append(hits, hit{at, end, a})
				}
			}
		}
		sort.Slice(hits, func(i, j int) bool { return hits[i].start < hits[j].start })
		for i, h := range hits {
			if len(out.Items) >= 20 {
				out.Warning = "一次最多整理 20 项；请检查原文中未列出的食材并分次录入。"
				return out
			}
			d := Draft{CatalogID: h.a.item.ID, Name: h.a.item.Name, Unit: h.a.item.Unit, Heard: clause}
			start, end := 0, len(clause)
			if i > 0 {
				start = hits[i-1].end
			}
			if i+1 < len(hits) {
				end = hits[i+1].start
			}
			before, after := clause[start:h.start], clause[h.end:end]
			found := amount.FindAllStringSubmatch(after, -1)
			trailingHalf := false
			// A suffix amount must immediately follow the name; otherwise it
			// may belong to the next ingredient ("番茄和六个鸡蛋").
			if loc := amount.FindStringIndex(after); loc == nil || strings.TrimSpace(after[:loc[0]]) != "" {
				found = nil
			} else {
				trailingHalf = strings.HasPrefix(strings.TrimSpace(after[loc[1]:]), "半")
			}
			all := amount.FindAllStringSubmatchIndex(before, -1)
			if len(all) > 0 {
				last := all[len(all)-1]
				// Do not reuse the amount immediately following the previous name.
				if strings.TrimSpace(before[last[1]:]) == "" && (i == 0 || strings.TrimSpace(before[:last[0]]) != "") {
					found = append(found, amount.FindAllStringSubmatch(before[last[0]:last[1]], -1)...)
				}
			}
			switch {
			case len(owners[h.a.name]) != 1:
				d.CatalogID = ""
				d.Warning = "名称匹配多个目录食材，请选择。"
			case strings.Contains(clause, "不对") || strings.Contains(clause, "不是") || strings.Contains(clause, "不要") || strings.Contains(clause, "没有") || strings.Contains(clause, "大概") || strings.Contains(clause, "差不多") || strings.Contains(clause, "左右"):
				d.Warning = "原文含修改、否定或估计，请核对数量或移除本项。"
			case len(found) != 1:
				d.Warning = "未找到唯一明确的数量与单位，请补充。"
			case trailingHalf:
				d.Warning = "原文包含附加的半份数量，请核对后填写完整数量。"
			default:
				q, err := convert(found[0][1], found[0][2], d.Unit)
				if err != nil {
					d.Warning = "数量或单位无法精确换算；包装、个数与重量不能自动互换。"
				} else {
					d.Quantity = core.Format(q)
				}
			}
			out.Items = append(out.Items, d)
		}
	}
	if len(out.Items) == 0 {
		out.Warning = "未匹配到目录食材；请核对转写原文并手动选择食材。"
	}
	return out
}

func convert(number, unit, target string) (int64, error) {
	decimal, err := chineseNumber(number)
	if err != nil {
		return 0, err
	}
	q, err := core.Quantity(decimal)
	if err != nil {
		return 0, err
	}
	factor := int64(1)
	switch unit {
	case "公斤", "千克":
		unit = "kg"
	case "克":
		unit = "g"
	case "毫升":
		unit = "ml"
	case "升":
		unit = "l"
	case "斤":
		unit = "g"
		factor = 500
	case "两":
		unit = "g"
		factor = 50
	case "枚":
		unit = "个"
	}
	if q > math.MaxInt64/factor {
		return 0, strconv.ErrRange
	}
	return core.Convert(q*factor, unit, target)
}

func chineseNumber(s string) (string, error) {
	if s == "半" {
		return "0.5", nil
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return s, nil
	}
	parts := strings.Split(s, "点")
	if len(parts) > 2 || len([]rune(s)) > 12 {
		return "", strconv.ErrSyntax
	}
	digits := map[rune]int64{'零': 0, '〇': 0, '一': 1, '二': 2, '两': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	integer := strings.Split(parts[0], "万")
	if len(integer) > 2 {
		return "", strconv.ErrSyntax
	}
	parseSection := func(s string) (int64, error) {
		if s == "零" || s == "〇" {
			return 0, nil
		}
		if !chineseSection.MatchString(s) || s == "" || strings.HasPrefix(s, "零") {
			return 0, strconv.ErrSyntax
		}
		var section, digit int64
		for _, r := range s {
			if v, ok := digits[r]; ok {
				digit = v
				continue
			}
			mult := map[rune]int64{'十': 10, '百': 100, '千': 1000}[r]
			if digit == 0 {
				digit = 1
			}
			section += digit * mult
			digit = 0
		}
		return section + digit, nil
	}
	n, err := parseSection(integer[0])
	if err != nil {
		return "", err
	}
	if len(integer) == 2 {
		if n == 0 {
			return "", strconv.ErrSyntax
		}
		n *= 10000
		if integer[1] != "" {
			tail, err := parseSection(strings.TrimPrefix(integer[1], "零"))
			if err != nil {
				return "", err
			}
			n += tail
		}
	}
	out := strconv.FormatInt(n, 10)
	if len(parts) == 2 {
		if parts[1] == "" {
			return "", strconv.ErrSyntax
		}
		out += "."
		for _, r := range parts[1] {
			v, ok := digits[r]
			if !ok {
				return "", strconv.ErrSyntax
			}
			out += strconv.FormatInt(v, 10)
		}
	}
	return out, nil
}

var chineseSection = regexp.MustCompile(`^(?:[一二两三四五六七八九]千)?(?:零?[一二两三四五六七八九]百)?(?:零?[一二两三四五六七八九]?十)?(?:零?[一二两三四五六七八九])?$`)
