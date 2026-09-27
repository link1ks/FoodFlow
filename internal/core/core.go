package core

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

func ID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func Hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func JSON(v any) []byte    { b, _ := json.Marshal(v); return b }
func Quantity(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "-") {
		return 0, errors.New("quantity must be positive")
	}
	p := strings.Split(s, ".")
	if len(p) > 2 || len(p[0]) > 15 {
		return 0, errors.New("invalid quantity")
	}
	whole, e := strconv.ParseInt(p[0], 10, 64)
	if e != nil {
		return 0, e
	}
	if whole > math.MaxInt64/1000 {
		return 0, errors.New("quantity overflow")
	}
	frac := int64(0)
	if len(p) == 2 {
		if len(p[1]) == 0 || len(p[1]) > 3 {
			return 0, errors.New("at most 3 decimal places")
		}
		f := p[1] + strings.Repeat("0", 3-len(p[1]))
		frac, e = strconv.ParseInt(f, 10, 64)
		if e != nil {
			return 0, e
		}
	}
	v := whole*1000 + frac
	if v <= 0 {
		return 0, errors.New("quantity must be positive")
	}
	return v, nil
}
func Format(q int64) string {
	sign := ""
	if q < 0 {
		sign = "-"
		q = -q
	}
	return fmt.Sprintf("%s%d.%03d", sign, q/1000, q%1000)
}
func Dimension(unit string) (string, int64, error) {
	switch unit {
	case "g":
		return "mass", 1, nil
	case "kg":
		return "mass", 1000, nil
	case "ml":
		return "volume", 1, nil
	case "l":
		return "volume", 1000, nil
	case "个", "只", "包", "袋", "盒", "份":
		return "count", 1, nil
	}
	return "", 0, fmt.Errorf("unsupported unit %q", unit)
}
func Convert(q int64, from, to string) (int64, error) {
	d1, f1, e := Dimension(from)
	if e != nil {
		return 0, e
	}
	d2, f2, e := Dimension(to)
	if e != nil {
		return 0, e
	}
	if d1 != d2 || (d1 == "count" && from != to) {
		return 0, errors.New("units cannot be converted without a confirmed conversion")
	}
	if q > math.MaxInt64/f1 {
		return 0, errors.New("quantity overflow")
	}
	n := q * f1
	if n%f2 != 0 {
		return 0, errors.New("conversion exceeds 3 decimal places")
	}
	return n / f2, nil
}
func Scale(q int64, servings, base int) (int64, error) {
	if q <= 0 || servings <= 0 || base <= 0 || servings > 20 {
		return 0, errors.New("invalid servings")
	}
	if q > (math.MaxInt64-int64(base)+1)/int64(servings) {
		return 0, errors.New("quantity overflow")
	}
	n := q * int64(servings)
	d := int64(base)
	return (n + d - 1) / d, nil
} // round up to 0.001
