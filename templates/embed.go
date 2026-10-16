// Package templates — тексты писем и уведомлений (07 §6: тексты хранятся в репозитории).
package templates

import "embed"

// FS — шаблоны.
//
//go:embed email/*.tmpl
var FS embed.FS
