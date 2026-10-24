package bridge

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/leha-creator/maxim/internal/integration/rocketwork"
	"github.com/leha-creator/maxim/internal/payout/domain"
	"github.com/leha-creator/maxim/internal/platform/logx"
	"github.com/leha-creator/maxim/internal/platform/money"
)

// ---------------------------------------------------------------- payout → Рокет Ворк

// RocketWorkAPI — методы адаптера Рокет Ворк: rocketwork.Client, rocketwork.Disabled,
// rocketworkfake.Fake.
type RocketWorkAPI interface {
	EnsureContractor(ctx context.Context, r rocketwork.ContractorRequest) (rocketwork.Contractor, error)
	GetContractor(ctx context.Context, id string) (rocketwork.Contractor, error)
	GetDepositBalance(ctx context.Context) (int64, error)
	CreatePayout(ctx context.Context, r rocketwork.PaymentRequest) (rocketwork.Payment, error)
	GetPayout(ctx context.Context, id string) (rocketwork.Payment, error)
	FindPayout(ctx context.Context, key string) (rocketwork.Payment, bool, error)
	ParseWebhook(header map[string][]string, body []byte) (rocketwork.Event, error)
}

// PayoutProvider — payout/domain.Provider поверх адаптера: типы и ошибки переносятся
// один в один (приведение статусов сделал адаптер, R-2).
type PayoutProvider struct {
	API RocketWorkAPI
	Log *slog.Logger
}

// unknown — WARN о незнакомом статусе сервиса (R-2): объект продолжают опрашивать.
func (p PayoutProvider) unknown(ctx context.Context, object, id, raw string) {
	if p.Log != nil {
		p.Log.WarnContext(ctx, "rocketwork unknown status", logx.KeyOp, "RocketWork.Status", "object", object,
			"external_id", id, "raw_status", raw)
	}
}

var _ domain.Provider = PayoutProvider{}

// providerErr переводит ошибки адаптера в ошибки домена; прочие (job.Retry предохранителя,
// отмена context) — как есть.
func providerErr(err error) error {
	var rej *rocketwork.RejectedError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &rej):
		return &domain.RejectedError{Code: domain.FailureCode(rej.Code), Message: rej.Message}
	case errors.Is(err, rocketwork.ErrDisabled):
		return domain.ErrProviderDisabled
	case errors.Is(err, rocketwork.ErrNotFound):
		return domain.ErrProviderNotFound
	case errors.Is(err, rocketwork.ErrSignature):
		return domain.ErrWebhookAuth
	case errors.Is(err, rocketwork.ErrFormat):
		return fmt.Errorf("%w: %v", domain.ErrWebhookFormat, err)
	case errors.Is(err, rocketwork.ErrUnavailable):
		return fmt.Errorf("%w: %v", domain.ErrProviderUnavailable, err)
	}
	return err
}

func contractorOf(c rocketwork.Contractor) domain.ProviderContractor {
	return domain.ProviderContractor{ExternalID: c.ID, Status: domain.ContractorStatus(c.Status),
		BlockedReason: c.BlockedReason, OnboardingURL: c.OnboardingURL}
}

func payoutOf(p rocketwork.Payment) domain.ProviderPayout {
	return domain.ProviderPayout{ExternalID: p.ID, Status: domain.Status(p.Status), Fee: money.Kopecks(p.FeeKopecks),
		ReceiptURL: p.ReceiptURL, FailureCode: domain.FailureCode(p.FailureCode), FailureMessage: p.FailureMessage}
}

func legalForm(t domain.LegalType) string {
	if t == domain.SoleProprietor {
		return rocketwork.LegalFormIndividual
	}
	return rocketwork.LegalFormSelfEmployed
}

func (p PayoutProvider) EnsureContractor(ctx context.Context, r domain.ContractorRequest) (domain.ProviderContractor, error) {
	c, err := p.API.EnsureContractor(ctx, rocketwork.ContractorRequest{ExternalID: r.OrganizationID.String(),
		INN: r.INN, FullName: r.FullName, Phone: r.Phone, LegalForm: legalForm(r.LegalType)})
	if err != nil {
		return domain.ProviderContractor{}, providerErr(err)
	}
	return contractorOf(c), nil
}

func (p PayoutProvider) GetContractor(ctx context.Context, externalID string) (domain.ProviderContractor, error) {
	c, err := p.API.GetContractor(ctx, externalID)
	if err != nil {
		return domain.ProviderContractor{}, providerErr(err)
	}
	if !c.Known {
		p.unknown(ctx, "contractor", c.ID, c.RawStatus)
	}
	return contractorOf(c), nil
}

func (p PayoutProvider) GetDepositBalance(ctx context.Context) (money.Kopecks, error) {
	b, err := p.API.GetDepositBalance(ctx)
	return money.Kopecks(b), providerErr(err)
}

func (p PayoutProvider) CreatePayout(ctx context.Context, o domain.PayoutOrder) (domain.ProviderPayout, error) {
	r, err := p.API.CreatePayout(ctx, rocketwork.PaymentRequest{IdempotencyKey: o.IdempotencyKey,
		ExternalID: o.PayoutID.String(), ContractorID: o.ContractorExternal, AmountKopecks: int64(o.Amount), Purpose: o.Purpose})
	if err != nil {
		return domain.ProviderPayout{}, providerErr(err)
	}
	return payoutOf(r), nil
}

func (p PayoutProvider) GetPayout(ctx context.Context, externalID string) (domain.ProviderPayout, error) {
	r, err := p.API.GetPayout(ctx, externalID)
	if err != nil {
		return domain.ProviderPayout{}, providerErr(err)
	}
	if !r.Known {
		p.unknown(ctx, "payout", r.ID, r.RawStatus)
	}
	return payoutOf(r), nil
}

func (p PayoutProvider) FindPayout(ctx context.Context, key string) (domain.ProviderPayout, bool, error) {
	r, found, err := p.API.FindPayout(ctx, key)
	if err != nil || !found {
		return domain.ProviderPayout{}, false, providerErr(err)
	}
	return payoutOf(r), true, nil
}

func (p PayoutProvider) ParseWebhook(header map[string][]string, body []byte) (domain.WebhookEvent, error) {
	ev, err := p.API.ParseWebhook(header, body)
	if err != nil {
		return domain.WebhookEvent{}, providerErr(err)
	}
	return domain.WebhookEvent{Kind: domain.WebhookKind(ev.Kind), ExternalID: ev.ObjectID, EventID: ev.EventID}, nil
}
