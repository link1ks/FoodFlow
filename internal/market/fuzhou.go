package market

import (
	"bytes"
	"fmt"
	"golang.org/x/net/html"
	"math/big"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const fuzhouIndex = "https://fgw.fuzhou.gov.cn/fgwzwgk/fzgggz/jgysf/"

var fuzhouDate = regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日福州市主副食品集超均价表`)

func fuzhouURL(body []byte) (string, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	var found string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Data == "a" && fuzhouDate.MatchString(content(n)) && found == "" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					base, _ := url.Parse(fuzhouIndex)
					u, e := base.Parse(a.Val)
					if e == nil && u.Scheme == "https" && u.Host == base.Host && strings.Contains(u.Path, "/msspjgxq/zfsp/") {
						found = u.String()
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if found == "" {
		return "", fmt.Errorf("Fuzhou report link missing")
	}
	return found, nil
}

func ParseFuzhou(body []byte, source string, now time.Time) ([]RegionalQuote, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	d := fuzhouDate.FindStringSubmatch(content(doc))
	if d == nil {
		return nil, fmt.Errorf("Fuzhou monitoring date missing")
	}
	y, _ := strconv.Atoi(d[1])
	m, _ := strconv.Atoi(d[2])
	day, _ := strconv.Atoi(d[3])
	date := fmt.Sprintf("%04d-%02d-%02d", y, m, day)
	ts, err := readTables(body, true)
	if err != nil {
		return nil, err
	}
	var out []RegionalQuote
	alias := map[string]string{"晚籼米": "大米", "粳米": "大米", "富强粉": "面粉", "标准粉": "面粉", "花生油": "食用油", "菜籽油": "食用油", "大豆油": "食用油", "调和油": "食用油", "猪瘦肉": "猪肉", "猪肋条肉": "猪肉", "普通鸡蛋": "鸡蛋", "普通鸭蛋": "鸭蛋", "冻带鱼": "带鱼", "活明虾": "虾", "活花蛤": "贝类", "红萝卜": "胡萝卜", "圆白菜": "卷心菜", "西红柿": "番茄", "脐橙": "橙子"}
	for _, t := range ts {
		if len(t) == 0 || strings.Join(t[0], "") != "类别商品名称规格等级计量单位今日集超均价" {
			continue
		}
		for _, r := range t[1:] {
			// Only the category column is row-spanned; the trailing four cells are stable.
			if len(r) != 4 && len(r) != 5 {
				return nil, fmt.Errorf("changed Fuzhou table row")
			}
			r = r[len(r)-4:]
			name, spec, unit, value := r[0], r[1], r[2], r[3]
			if value == "" || value == "-" {
				continue
			}
			if unit != "元/500克" && unit != "元/5升" {
				continue
			}
			q, err := quote(name, spec, value, date, now)
			if err != nil {
				return nil, err
			}
			price := q.Price
			if unit == "元/5升" {
				p, _ := new(big.Rat).SetString(value)
				price = p.Quo(p, big.NewRat(5, 1)).FloatString(2)
				unit = "1l"
				spec += "（原5升包装，折算每升）"
			} else {
				unit = "500g"
			}
			canonical := name
			if v := alias[name]; v != "" {
				canonical = v
			}
			out = append(out, RegionalQuote{"福州市", canonical, name, "集超零售均价 · " + spec, price, unit, "福州市发展和改革委员会", source, q.Day, q.Day})
		}
	}
	if len(out) < 20 {
		return nil, fmt.Errorf("Fuzhou price table missing")
	}
	return out, nil
}
