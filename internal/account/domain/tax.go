package domain

import "slices"

// Допустимые сочетания типа лица, режима и ставки НДС — рабочие таблицы 04 §2.
// Проверить с бухгалтером. Q-37: при смене ответа правятся только эти таблицы.
var (
	allowedTaxRegimes = map[LegalType][]TaxRegime{
		SelfEmployed:   {TaxNPD},
		SoleProprietor: {TaxOSNO, TaxUSN, TaxAUSN, TaxESHN, TaxPSN, TaxNPD},
		LegalEntity:    {TaxOSNO, TaxUSN, TaxAUSN, TaxESHN},
	}
	allowedVatRates = map[TaxRegime][]VatRate{
		TaxOSNO: {Vat22, VatNone},
		TaxUSN:  {VatNone, Vat5, Vat7, Vat22},
		TaxESHN: {Vat22, VatNone},
		TaxAUSN: {VatNone},
		TaxPSN:  {VatNone},
		TaxNPD:  {VatNone},
	}
)

// TaxRegimeAllowed сообщает, что режим допустим для типа лица (Q-37).
func TaxRegimeAllowed(t LegalType, r TaxRegime) bool {
	return slices.Contains(allowedTaxRegimes[t], r)
}

// VatRateAllowed сообщает, что ставка допустима для режима (Q-37).
func VatRateAllowed(r TaxRegime, v VatRate) bool {
	return slices.Contains(allowedVatRates[r], v)
}
