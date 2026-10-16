package main

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestObjectIORejectsPooledRequestContext(t *testing.T) {
	for _, tc := range []struct {
		source string
		reject bool
	}{
		{`package app; import "github.com/gin-gonic/gin";func upload(c *gin.Context){store.Put(c,"key",nil,"image/png")}`, true},
		{`package app; import g "github.com/gin-gonic/gin";func preview(req *g.Context){store.Open(req,"key")}`, true},
		{`package app; import "github.com/gin-gonic/gin";func cancel(c *gin.Context){defer func(){store.Delete(c,"key")}()}`, true},
		{`package app; import "github.com/gin-gonic/gin";func upload(c *gin.Context){store.Put(c.Request.Context(),"key",nil,"image/png")}`, false},
		{`package app; import "context";func worker(ctx context.Context){store.Open(ctx,"key")}`, false},
	} {
		f, err := parser.ParseFile(token.NewFileSet(), "fixture.go", tc.source, 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := pooledObjectContextViolation(f) != ""; got != tc.reject {
			t.Fatalf("reject=%v want %v: %s", got, tc.reject, tc.source)
		}
	}
}
