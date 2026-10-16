package domain

import "errors"

// RuleError — нарушение правила с машинной причиной: app передаёт её в
// details.reason ответа 422. Сравнивается по указателю (errors.Is).
type RuleError struct {
	// Field — поле запроса, к которому относится причина.
	Field  string
	Reason string
}

func (e *RuleError) Error() string { return "post: " + e.Field + ": " + e.Reason }

// ErrNotFound — объекта нет или он чужой; репозитории возвращают его, app переводит в 404.
var ErrNotFound = errors.New("post: not found")

// ErrUndecodable — изображение не декодируется: повтор не поможет.
var ErrUndecodable = errors.New("post: image cannot be decoded")

// ErrDerivativeTooLarge — производная не помещается в предел MAX даже после перекодирования в JPEG.
var ErrDerivativeTooLarge = errors.New("post: derivative exceeds the size limit")

// ErrStaleVersion — версия поста изменена параллельно (lock_version, 04 §1).
var ErrStaleVersion = errors.New("post: stale lock_version")
