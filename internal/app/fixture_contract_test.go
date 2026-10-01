package app

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
)

var fixtureContract struct {
	sync.Once
	router routers.Router
	err    error
}

// Common API fixtures validate actual responses, including expected rejections.
// Deliberately invalid request fixtures must continue reaching real handlers.
func validateFixtureResponse(t *testing.T, req *http.Request, res *http.Response, payload []byte) {
	t.Helper()
	fixtureContract.Do(func() {
		var doc *openapi3.T
		doc, fixtureContract.err = openapi3.NewLoader().LoadFromFile(filepath.Join("..", "..", "openapi.yaml"))
		if fixtureContract.err == nil {
			doc.Servers = nil
			fixtureContract.router, fixtureContract.err = legacy.NewRouter(doc)
		}
	})
	if fixtureContract.err != nil {
		t.Fatal(fixtureContract.err)
	}
	copied := req.Clone(context.Background())
	copied.URL.Path = strings.TrimPrefix(copied.URL.Path, "/api")
	route, params, err := fixtureContract.router.FindRoute(copied)
	if err != nil {
		t.Fatal(err)
	}
	input := &openapi3filter.RequestValidationInput{Request: copied, PathParams: params, Route: route, Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc, IncludeResponseStatus: true}}
	response := &openapi3filter.ResponseValidationInput{RequestValidationInput: input, Status: res.StatusCode, Header: res.Header, Body: io.NopCloser(bytes.NewReader(payload))}
	if err = openapi3filter.ValidateResponse(context.Background(), response); err != nil {
		t.Fatalf("fixture response %s %s (%d): %v", req.Method, copied.URL.Path, res.StatusCode, err)
	}
}

func TestAllSuccessfulContractResponsesHaveTypedMedia(t *testing.T) {
	doc := loadContract(t)
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			for code, res := range op.Responses.Map() {
				if strings.HasPrefix(code, "2") && code != "204" {
					if len(res.Value.Content) == 0 {
						t.Errorf("%s %s %s has no response media", method, path, code)
					}
					for _, media := range res.Value.Content {
						if media.Schema == nil {
							t.Errorf("%s %s has no schema", method, path)
						}
					}
				}
			}
		}
	}
	schema := doc.Components.Schemas["NutritionBasis"].Value
	if err := schema.VisitJSON(map[string]any{"recorded": false, "revision": float64(0)}); err != nil {
		t.Fatal(err)
	}
	if err := schema.VisitJSON(map[string]any{"recorded": true, "revision": float64(1), "quantity": float64(2)}); err == nil {
		t.Fatal("numeric sample quantity accepted")
	}
}
