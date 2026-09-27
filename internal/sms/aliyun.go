package sms

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"foodflow/internal/core"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Sender interface {
	Send(context.Context, string, string) error
}
type Aliyun struct {
	KeyID, Secret, Sign, Template string
	Client                        *http.Client
}

func FromEnv() Sender {
	if os.Getenv("SMS_PROVIDER") != "aliyun" {
		return nil
	}
	s := Aliyun{KeyID: os.Getenv("SMS_ACCESS_KEY_ID"), Secret: os.Getenv("SMS_ACCESS_KEY_SECRET"), Sign: os.Getenv("SMS_SIGN_NAME"), Template: os.Getenv("SMS_TEMPLATE_CODE")}
	if s.KeyID == "" || s.Secret == "" || s.Sign == "" || s.Template == "" {
		return nil
	}
	return s
}
func escape(s string) string { return strings.ReplaceAll(url.QueryEscape(s), "+", "%20") }
func (s Aliyun) Send(ctx context.Context, phone, code string) error {
	p := url.Values{"Action": {"SendSms"}, "Version": {"2017-05-25"}, "Format": {"JSON"}, "RegionId": {"cn-hangzhou"}, "AccessKeyId": {s.KeyID}, "SignatureMethod": {"HMAC-SHA1"}, "SignatureVersion": {"1.0"}, "SignatureNonce": {core.ID()}, "Timestamp": {time.Now().UTC().Format("2006-01-02T15:04:05Z")}, "PhoneNumbers": {strings.TrimPrefix(phone, "+86")}, "SignName": {s.Sign}, "TemplateCode": {s.Template}, "TemplateParam": {string(core.JSON(map[string]string{"code": code}))}}
	canonical := strings.ReplaceAll(p.Encode(), "+", "%20")
	mac := hmac.New(sha1.New, []byte(s.Secret+"&"))
	mac.Write([]byte("POST&%2F&" + escape(canonical)))
	p.Set("Signature", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	req, err := http.NewRequestWithContext(ctx, "POST", "https://dysmsapi.aliyuncs.com/", strings.NewReader(p.Encode()))
	if err != nil {
		return errors.New("短信请求构造失败")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("短信发送未确认，请稍后再试")
	}
	defer resp.Body.Close()
	var out struct{ Code string }
	if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&out) != nil || out.Code != "OK" {
		return errors.New("短信服务拒绝发送，请检查签名、模板及额度")
	}
	return nil
}
