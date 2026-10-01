package app

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func loadContract(t *testing.T) *openapi3.T {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join("..", "..", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err = doc.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return doc
}

var routeParameter = regexp.MustCompile(`:([a-zA-Z_][a-zA-Z0-9_]*)`)

func contractRoutesMatch(doc *openapi3.T, actual map[string]bool) error {
	documented := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			documented[method+" "+path] = true
		}
	}
	for route := range actual {
		if !documented[route] {
			return fmt.Errorf("undocumented API route: %s", route)
		}
	}
	for route := range documented {
		if !actual[route] {
			return fmt.Errorf("OpenAPI route has no implementation: %s", route)
		}
	}
	return nil
}

func TestOpenAPIDocument(t *testing.T) {
	doc := loadContract(t)
	actual := map[string]bool{}
	for _, route := range (&App{}).Router().Routes() {
		if strings.HasPrefix(route.Path, "/api/") {
			path := routeParameter.ReplaceAllString(strings.TrimPrefix(route.Path, "/api"), "{$1}")
			actual[route.Method+" "+path] = true
		}
	}
	if err := contractRoutesMatch(doc, actual); err != nil {
		t.Fatal(err)
	}
	delete(actual, http.MethodGet+" /ingredient-catalog")
	if err := contractRoutesMatch(doc, actual); err == nil {
		t.Fatal("deleted implementation accepted")
	}
	actual[http.MethodGet+" /ingredient-catalog"] = true
	actual[http.MethodPost+" /undocumented"] = true
	if err := contractRoutesMatch(doc, actual); err == nil {
		t.Fatal("undocumented implementation accepted")
	}
}

func TestContractRejectsQuantityAndRequiredFieldDrift(t *testing.T) {
	doc := loadContract(t)
	schema := doc.Components.Schemas["InventoryItem"].Value
	valid := map[string]any{"id": "10000000-0000-4000-8000-000000000001", "name": "番茄", "category": "蔬菜", "unit": "g", "dimension": "mass", "quantity": "300.000", "low": "0.000", "low_stock": false, "has_image": false, "image_version": float64(0)}
	if err := schema.VisitJSON(valid); err != nil {
		t.Fatal(err)
	}
	valid["quantity"] = float64(300)
	if err := schema.VisitJSON(valid); err == nil {
		t.Fatal("numeric quantity accepted")
	}
	valid["quantity"] = "300.000"
	delete(valid, "unit")
	if err := schema.VisitJSON(valid); err == nil {
		t.Fatal("missing required unit accepted")
	}
}
