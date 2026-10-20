package domain

import "errors"

var (
	// ErrNotFound — записи нет или она чужая.
	ErrNotFound = errors.New("billing: not found")
	// ErrAmountOutOfRange — сумма счёта вне пределов.
	ErrAmountOutOfRange = errors.New("billing: invoice amount is out of range")
	// ErrInvalidTransition — смена состояния счёта или поступления недопустима.
	ErrInvalidTransition = errors.New("billing: invalid transition")
	// ErrStatementImported — этот файл выписки уже загружен (R-12).
	ErrStatementImported = errors.New("billing: statement already imported")
	// ErrStatementAccount — выписка не по расчётному счёту платформы (R-12).
	ErrStatementAccount = errors.New("billing: statement is for another account")
	// ErrStatementFormat — файл выписки не разобран.
	ErrStatementFormat = errors.New("billing: statement format error")
	// ErrStatementTooLarge — файл выписки больше STATEMENT_MAX_BYTES.
	ErrStatementTooLarge = errors.New("billing: statement file is too large")
)
