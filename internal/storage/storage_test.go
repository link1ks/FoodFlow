package storage

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestLocal(t *testing.T) {
	s := Local{Root: t.TempDir()}
	ctx := context.Background()
	if e := s.Put(ctx, "../escape", strings.NewReader("bad"), "text/plain"); e == nil {
		t.Fatal("path traversal accepted")
	}
	if e := s.Put(ctx, "house/photo.jpg", strings.NewReader("abc"), "image/jpeg"); e != nil {
		t.Fatal(e)
	}
	r, e := s.Open(ctx, "house/photo.jpg")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(r)
	r.Close()
	if string(b) != "abc" {
		t.Fatal(string(b))
	}
	if e = s.Delete(ctx, "house/photo.jpg"); e != nil {
		t.Fatal(e)
	}
}
