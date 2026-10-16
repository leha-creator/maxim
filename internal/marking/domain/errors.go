package domain

import "errors"

var (
	// ErrNotFound — записи нет.
	ErrNotFound = errors.New("marking: not found")
	// ErrNameTooLong — наименование организации длиннее 255 символов (R-6).
	ErrNameTooLong = errors.New("marking: organization name is too long for ORD")
	// ErrOfferNotAccepted — сторона договора не принимала оферту (R-7).
	ErrOfferNotAccepted = errors.New("marking: offer is not accepted")
	// ErrPermanent — запрос нельзя собрать или ОРД отказал: повтор не поможет.
	ErrPermanent = errors.New("marking: permanent error")
)

// BuildFailureReason — причина error для ошибки сборки запроса (без вызова ОРД).
func BuildFailureReason(err error) (string, bool) {
	switch {
	case errors.Is(err, ErrNameTooLong):
		return ReasonNameTooLong, true
	case errors.Is(err, ErrOfferNotAccepted):
		return ReasonOfferNotAccepted, true
	case errors.Is(err, ErrPermanent):
		return err.Error(), true
	}
	return "", false
}
