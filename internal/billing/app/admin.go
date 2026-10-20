package app

import (
	"context"
	"errors"

	"github.com/leha-creator/maxim/internal/billing/domain"
	"github.com/leha-creator/maxim/internal/platform/money"
)

// InvoiceByNumber — счёт по номеру MX-… (служебные команды); нет — 404.
func (s *Service) InvoiceByNumber(ctx context.Context, number string) (domain.Invoice, error) {
	var inv domain.Invoice
	err := s.Tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		inv, err = s.Invoices.ByNumberForUpdate(ctx, number)
		return err
	})
	return inv, notFound(err, "invoice")
}

// OrganizationByINN — организация по ИНН (ручное сопоставление, R-13); нет — 404.
func (s *Service) OrganizationByINN(ctx context.Context, inn string) (domain.OrgRequisites, error) {
	org, err := s.Organizations.ByINN(ctx, inn)
	return org, notFound(err, "organization")
}

// BankPosition — данные банка для сверки (06 §2.4): последняя выписка и несопоставленные
// поступления.
type BankPosition struct {
	// Latest — последняя выписка; nil — выписок нет.
	Latest         *domain.BankStatement
	UnmatchedCount int
	UnmatchedTotal money.Kopecks
}

// BankPosition читает данные банка для сверки.
func (s *Service) BankPosition(ctx context.Context) (BankPosition, error) {
	var p BankPosition
	latest, err := s.Statements.Latest(ctx)
	switch {
	case err == nil:
		p.Latest = &latest
	case !errors.Is(err, domain.ErrNotFound):
		return p, err
	}
	p.UnmatchedCount, p.UnmatchedTotal, err = s.BankTransactions.UnmatchedTotal(ctx)
	return p, err
}
