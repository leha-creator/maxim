// Package templates — тексты писем и уведомлений (07 §6: тексты хранятся в репозитории).
package templates

import "embed"

// FS — шаблоны: письма (email/) и уведомления по кодам каталога (notification/).
//
//go:embed email/*.tmpl notification/*.tmpl
var FS embed.FS
