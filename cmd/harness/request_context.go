package main

import (
	"go/ast"
	"strconv"
)

// A Gin context is pooled and reset for the next request. An HTTP storage
// transport may still inspect its context after the handler has returned.
// This is a direct-argument guard, not a general data-flow/escape analysis.
func pooledObjectContextViolation(file *ast.File) string {
	ginAlias := ""
	for _, imp := range file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if path == "github.com/gin-gonic/gin" {
			ginAlias = "gin"
			if imp.Name != nil {
				ginAlias = imp.Name.Name
			}
		}
	}
	if ginAlias == "" {
		return ""
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		pooled := map[string]bool{}
		for _, param := range fn.Type.Params.List {
			pointer, ok := param.Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			typ, ok := pointer.X.(*ast.SelectorExpr)
			if !ok || typ.Sel.Name != "Context" {
				continue
			}
			pkg, ok := typ.X.(*ast.Ident)
			if !ok || pkg.Name != ginAlias {
				continue
			}
			for _, name := range param.Names {
				pooled[name.Name] = true
			}
		}
		violation := ""
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			method, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if method.Sel.Name != "Put" && method.Sel.Name != "Open" && method.Sel.Name != "Delete" {
				return true
			}
			arg, ok := call.Args[0].(*ast.Ident)
			if ok && pooled[arg.Name] {
				violation = "object I/O receives pooled Gin context; pass the immutable HTTP request context"
				return false
			}
			return true
		})
		if violation != "" {
			return violation
		}
	}
	return ""
}
