package sms

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestAliyunRequest(t *testing.T) {
	s := Aliyun{KeyID: "test-id", Secret: "test-secret", Sign: "食光", Template: "SMS_TEST"}
	calls := 0
	s.Client = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://dysmsapi.aliyuncs.com/" || r.Method != "POST" {
			t.Fatal("wrong destination")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("PhoneNumbers") != "13800138000" || r.Form.Get("TemplateParam") != "{\"code\":\"123456\"}" || r.Form.Get("Action") != "SendSms" {
			t.Fatal("incorrect SMS parameters")
		}
		signature := r.Form.Get("Signature")
		r.Form.Del("Signature")
		mac := hmac.New(sha1.New, []byte("test-secret&"))
		mac.Write([]byte("POST&%2F&" + escape(strings.ReplaceAll(r.Form.Encode(), "+", "%20"))))
		if signature != base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
			t.Fatal("invalid signature")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"Code":"OK"}`))}, nil
	})}
	if err := s.Send(context.Background(), "+8613800138000", "123456"); err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
	s.Client = &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"Code":"isv.BUSINESS_LIMIT_CONTROL"}`))}, nil
	})}
	if s.Send(context.Background(), "+8613800138000", "123456") == nil {
		t.Fatal("provider rejection accepted")
	}
}
