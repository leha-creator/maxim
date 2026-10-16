// Package ids создаёт идентификаторы записей: UUID версии 7 (04 §1).
package ids

import "github.com/google/uuid"

// New возвращает новый UUID v7. Генератор берёт время из системных часов:
// идентификатор упорядочивает записи, но не служит отметкой времени предметной области.
func New() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}
