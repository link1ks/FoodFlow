package storage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestS3SDKPutReadDeleteCompatibility(t *testing.T) {
	var calls atomic.Int32
	content := []byte("synthetic-photo-and-audio-bytes")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/fixture-bucket/house/voice.wav" {
			t.Errorf("unexpected object path: %s", r.URL.Path)
			w.WriteHeader(400)
			return
		}
		switch r.Method {
		case "PUT":
			body, err := io.ReadAll(io.LimitReader(r.Body, 1024))
			if err != nil || !bytes.Equal(body, content) || r.Header.Get("Content-Type") != "audio/wav" {
				t.Errorf("upload changed content or metadata")
				w.WriteHeader(400)
				return
			}
			w.Header().Set("ETag", `"fixture-etag"`)
		case "GET":
			w.Header().Set("Content-Type", "audio/wav")
			_, _ = w.Write(content)
		case "DELETE":
			w.WriteHeader(204)
		default:
			t.Errorf("unexpected method: %s", r.Method)
			w.WriteHeader(400)
		}
	}))
	defer server.Close()
	store, err := NewS3(server.URL, "fixture-region", "fixture-bucket", "fixture-access", "fixture-secret")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := store.Put(ctx, "house/voice.wav", bytes.NewReader(content), "audio/wav"); err != nil {
		t.Fatal(err)
	}
	reader, err := store.Open(ctx, "house/voice.wav")
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || !bytes.Equal(got, content) {
		t.Fatal("download changed object bytes")
	}
	if err := store.Delete(ctx, "house/voice.wav"); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ctx, "../escape", bytes.NewReader(content), "audio/wav"); err == nil {
		t.Fatal("invalid key accepted")
	}
	if calls.Load() != 3 {
		t.Fatalf("unexpected requests: %d", calls.Load())
	}
}

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOSSObjectAddressingCompatibility(t *testing.T) {
	content := []byte("synthetic-private-object")
	store, err := NewS3("https://s3.oss-cn-beijing.aliyuncs.com", "cn-beijing", "fixture-bucket", "fixture-access", "fixture-secret")
	if err != nil {
		t.Fatal(err)
	}
	var calls int
	store.Client = s3.New(store.Client.Options(), func(o *s3.Options) {
		o.HTTPClient = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.URL.Host != "fixture-bucket.s3.oss-cn-beijing.aliyuncs.com" || r.URL.Path != "/house/photo.webp" {
				t.Errorf("OSS requires bucket hostname and object-only path: %s", r.URL)
			}
			body := []byte{}
			status := 200
			switch r.Method {
			case "PUT":
				got, err := io.ReadAll(r.Body)
				if err != nil || !bytes.Equal(got, content) || r.Header.Get("Content-Type") != "image/webp" {
					t.Error("upload bytes or MIME changed")
				}
			case "GET":
				body = content
			case "DELETE":
				status = 204
			default:
				t.Errorf("unexpected method %s", r.Method)
			}
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body)), Request: r}, nil
		})}
	})
	ctx := context.Background()
	if err := store.Put(ctx, "house/photo.webp", bytes.NewReader(content), "image/webp"); err != nil {
		t.Fatal(err)
	}
	r, err := store.Open(ctx, "house/photo.webp")
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	r.Close()
	if err != nil || !bytes.Equal(got, content) {
		t.Fatal("read bytes changed")
	}
	if err := store.Delete(ctx, "house/photo.webp"); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("unexpected requests: %d", calls)
	}
	for _, endpoint := range []string{"http://127.0.0.1:9000", "https://s3.oss-cn-beijing.aliyuncs.com.example.org", "https://oss-cn-beijing.aliyuncs.com"} {
		other, err := NewS3(endpoint, "fixture-region", "fixture-bucket", "fixture-access", "fixture-secret")
		if err != nil || !other.Client.Options().UsePathStyle {
			t.Errorf("path-style compatibility lost for %s", endpoint)
		}
	}
}
