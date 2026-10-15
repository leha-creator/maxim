package api

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// TestSpecIsValid проверяет исходный файл спецификации, а не только встроенную копию:
// если api.gen.go не перегенерирован после правки, тест укажет на расхождение.
func TestSpecIsValid(t *testing.T) {
	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if err := spec.Validate(context.Background()); err != nil {
		t.Fatalf("spec is invalid: %v", err)
	}
	embedded, err := GetSwagger()
	if err != nil {
		t.Fatalf("embedded spec: %v", err)
	}
	if got, want := len(embedded.Paths.Map()), len(spec.Paths.Map()); got != want {
		t.Fatalf("embedded spec has %d paths, file has %d: run go generate ./...", got, want)
	}
}
