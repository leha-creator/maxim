package domain

import "errors"

var (
	// ErrNotFound — уведомления нет или оно чужое.
	ErrNotFound = errors.New("notification: not found")
	// ErrUnknownTemplate — кода нет в каталоге (ошибка программы).
	ErrUnknownTemplate = errors.New("notification: unknown template")
	// ErrRoleNotAllowed — шаблон не предназначен этой стороне (ошибка программы).
	ErrRoleNotAllowed = errors.New("notification: template is not for this role")
	// ErrNotReadable — прочитанным отмечается только уведомление кабинета.
	ErrNotReadable = errors.New("notification: only cabinet notifications can be read")
)
