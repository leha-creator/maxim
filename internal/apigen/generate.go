// Package apigen содержит код, сгенерированный из api/openapi.yaml: модели,
// интерфейс strict server и встроенную спецификацию. Руками не править.
//
// После генерации genstub пересоздаёт internal/bootstrap/unimplemented.go —
// ответ 501 для операций, которые ещё не реализованы (R-7).
package apigen

//go:generate go tool oapi-codegen -config oapi-codegen.yaml ../../api/openapi.yaml
//go:generate go run ./cmd/genstub -in api.gen.go -out ../bootstrap/unimplemented.go
