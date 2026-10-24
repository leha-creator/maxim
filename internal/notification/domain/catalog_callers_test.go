package domain

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCatalogCodesHaveCallers — у каждого кода каталога есть вызывающий вне модуля
// уведомлений (строка кода в исходниках internal/), кроме кодов с AwaitsModule (R-11):
// у них вызывающих ещё нет, и появление вызова требует снять метку.
func TestCatalogCodesHaveCallers(t *testing.T) {
	root := filepath.Join("..", "..")
	var src strings.Builder
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "notification" || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		src.Write(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	all := src.String()
	for _, tpl := range Catalog() {
		called := strings.Contains(all, `"`+tpl.Code+`"`)
		switch {
		case tpl.AwaitsModule == "" && !called:
			t.Errorf("%s: no caller outside notification", tpl.Code)
		case tpl.AwaitsModule != "" && called:
			t.Errorf("%s: has a caller, drop AwaitsModule %q", tpl.Code, tpl.AwaitsModule)
		}
	}
}
