package domain

import "github.com/google/uuid"

// MatchMethod — как сопоставлено поступление (журнал и итог загрузки).
type MatchMethod string

const (
	MatchByNumber    MatchMethod = "number"
	MatchByINNAmount MatchMethod = "inn_amount"
	MatchManually    MatchMethod = "manual"
)

// Candidates — счета-кандидаты для поступления: по номерам из назначения и выставленные
// с ИНН плательщика и той же суммой.
type Candidates struct {
	ByNumber       []Invoice
	ByINNAndAmount []Invoice
}

// MatchResult — итог сопоставления. Matched = false — поступление ждёт администратора.
type MatchResult struct {
	Matched        bool
	Invoice        *Invoice
	OrganizationID uuid.UUID
	Method         MatchMethod
}

// Match — шаги 2–4 алгоритма 05 §5 (шаг 1, повтор операции, проверяет хранилище):
//  2. номер счёта из назначения платежа, счёт issued или expired;
//  3. иначе — единственный issued-счёт с ИНН плательщика и той же суммой;
//  4. иначе — не сопоставлено.
//
// Отменённый счёт не сопоставляется (R-11).
func Match(in Incoming, c Candidates) MatchResult {
	numbers := map[string]bool{}
	for _, n := range FindNumbers(in.Purpose) {
		numbers[n] = true
	}
	for i := range c.ByNumber {
		inv := &c.ByNumber[i]
		if numbers[inv.Number] && inv.Payable() {
			return MatchResult{Matched: true, Invoice: inv, OrganizationID: inv.OrganizationID, Method: MatchByNumber}
		}
	}
	if in.PayerINN == "" {
		return MatchResult{}
	}
	var found *Invoice
	for i := range c.ByINNAndAmount {
		inv := &c.ByINNAndAmount[i]
		if inv.Status != InvoiceIssued || inv.Payer.INN != in.PayerINN || inv.Amount != in.Amount {
			continue
		}
		if found != nil {
			return MatchResult{} // два кандидата — решает администратор
		}
		found = inv
	}
	if found == nil {
		return MatchResult{}
	}
	return MatchResult{Matched: true, Invoice: found, OrganizationID: found.OrganizationID, Method: MatchByINNAmount}
}
