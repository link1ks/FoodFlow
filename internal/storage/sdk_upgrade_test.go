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
