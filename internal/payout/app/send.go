package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/leha-creator/maxim/internal/payout/domain"
	"github.com/leha-creator/maxim/internal/platform/alert"
	"github.com/leha-creator/maxim/internal/platform/job"
	"github.com/leha-creator/maxim/internal/platform/logx"
)

// Send — задание send_payout (05 §4.3, R-6, R-7): проверки и CreatePayout вне транзакции,
// затем отметка sent и опрос в транзакции. attempt — номер выполнения задания.
func (s *Service) Send(ctx context.Context, payoutID uuid.UUID, attempt int) error {
	const op = "Payout.Send"
	p, err := s.Store.PayoutByID(ctx, payoutID, false)
	if errors.Is(err, domain.ErrNotFound) {
		return job.Permanent(err)
	}
	if err != nil {
		return err
	}
	if p.Status != domain.StatusCreated || p.Method != domain.MethodRocketwork {
		s.log.DebugContext(ctx, "payout send skipped", logx.KeyOp, op, KeyPayoutID, p.ID, "status", p.Status, "method", p.Method)
		return nil
	}
	log := s.log.With(logx.KeyOp, op, KeyPayoutID, p.ID, KeyPlacementID, p.PlacementID, KeyAttempt, p.Attempt,
		"job_attempt", attempt, KeyIdempotencyKey, p.IdempotencyKey)

	// R-7: исполнитель active по свежему статусу.
	c, err := s.Store.ContractorByID(ctx, p.ContractorID, false)
	if err != nil {
		return err
	}
	if c.Registered() && (c.Stale(s.Clock.Now(), s.Config.StatusMaxAge) || !c.CanAccept()) {
		pc, err := s.Provider.GetContractor(ctx, c.ExternalID)
		if err != nil {
			return s.providerWait(ctx, op, err)
		}
		if c, err = s.applyContractor(ctx, c.ID, pc); err != nil {
			return err
		}
	}
	if !c.CanAccept() {
		code := domain.FailureContractorNotActive
		if c.BlockedReason == string(domain.FailureTaxStatusLost) {
			code = domain.FailureTaxStatusLost
		}
		return s.fail(ctx, p.ID, code, "contractor is "+string(c.Status))
	}

	// Депозит не меньше суммы; иначе ждём час, попытка не тратится (05 §4.3).
	deposit, err := s.Provider.GetDepositBalance(ctx)
	if err != nil {
		return s.providerWait(ctx, op, err)
	}
	s.Metrics.Deposit("api", deposit)
	if deposit < p.Amount {
		return s.depositShort(ctx, p, int64(deposit))
	}

	brief, err := s.Placements.Brief(ctx, []uuid.UUID{p.PlacementID})
	if err != nil {
		return err
	}
	order := domain.PayoutOrder{IdempotencyKey: p.IdempotencyKey, PayoutID: p.ID, ContractorExternal: c.ExternalID,
		Amount: p.Amount, Purpose: purpose(brief[p.PlacementID])}
	pp, err := s.Provider.CreatePayout(ctx, order)
	if err != nil {
		if rej, ok := domain.AsRejected(err); ok {
			if rej.Code == domain.FailureInsufficientDeposit {
				return s.depositShort(ctx, p, int64(deposit))
			}
			return s.fail(ctx, p.ID, rej.Code, rej.Message)
		}
		if !errors.Is(err, domain.ErrProviderUnavailable) {
			return s.providerWait(ctx, op, err)
		}
		// Неизвестный исход (R-6): ищем выплату по нашему ключу, иначе повтор с тем же ключом.
		found, ok, ferr := s.Provider.FindPayout(ctx, p.IdempotencyKey)
		if ferr != nil || !ok {
			log.WarnContext(ctx, "payout send retry", logx.KeyError, err.Error(), "found", ok)
			return err
		}
		log.InfoContext(ctx, "payout found after unknown outcome", KeyExternalID, found.ExternalID)
		pp = found
	}
	if err := s.markSent(ctx, p.ID, pp.ExternalID); err != nil {
		return err
	}
	log.InfoContext(ctx, "payout sent", KeyExternalID, pp.ExternalID, KeyAmount, int64(p.Amount))
	s.notify(ctx, domain.Notice{Template: domain.NoticePayoutSent, OrganizationID: p.BloggerOrganizationID, ObjectID: p.ID,
		Data: map[string]any{"amount": int64(p.Amount)}})
	s.resolve(ctx, AlertDepositInsufficient, payoutObject(p.ID))
	_, err = s.apply(ctx, p.ID, pp)
	return err
}

func purpose(b domain.PlacementBrief) string {
	if b.OrderNumber == 0 {
		return "Вознаграждение за размещение рекламы"
	}
	return fmt.Sprintf("Вознаграждение за размещение рекламы, заказ №%d, канал «%s»", b.OrderNumber, b.ChannelTitle)
}

// depositShort — депозита не хватает: выплата остаётся created, ERROR и оповещение, повтор через час.
func (s *Service) depositShort(ctx context.Context, p domain.Payout, deposit int64) error {
	s.log.ErrorContext(ctx, "rocketwork deposit insufficient", logx.KeyOp, "Payout.Send", KeyPayoutID, p.ID,
		"deposit", deposit, "required", int64(p.Amount), "retry_in", s.Config.DepositRetry.String())
	s.raise(ctx, alert.LevelError, AlertDepositInsufficient, payoutObject(p.ID), "rocketwork deposit is not enough for a payout",
		map[string]any{"payout_id": p.ID.String(), "deposit": deposit, "required": int64(p.Amount)})
	return job.Postpone(fmt.Errorf("%w: deposit %d < %d", errDepositShort, deposit, p.Amount), s.Config.DepositRetry)
}

var errDepositShort = errors.New("rocketwork deposit is insufficient")

// markSent — отметка sent и опрос (R-10) в одной транзакции.
func (s *Service) markSent(ctx context.Context, id uuid.UUID, externalID string) error {
	return s.Tx.InTx(ctx, func(ctx context.Context) error {
		p, err := s.Store.PayoutByID(ctx, id, true)
		if err != nil {
			return err
		}
		changed, err := p.MarkSent(externalID, s.Clock.Now())
		if err != nil || !changed {
			return err
		}
		if err := s.Store.UpdatePayout(ctx, &p); err != nil {
			return err
		}
		return s.Jobs.Enqueue(ctx, job.Job{Kind: KindPollPayout, DedupeKey: PollPayoutKey(p.ID),
			Payload: PayoutPayload{PayoutID: p.ID}, RunAt: s.Clock.Now().Add(s.Config.PollFast), MaxAttempts: PollAttempts})
	})
}

// SendExhausted — попытки send_payout исчерпаны (R-8): failed provider_error и оповещение.
func (s *Service) SendExhausted(ctx context.Context, payoutID uuid.UUID, cause error) {
	s.log.ErrorContext(ctx, "payout send exhausted", logx.KeyOp, "Payout.SendExhausted", KeyPayoutID, payoutID,
		logx.KeyError, cause.Error())
	if err := s.fail(ctx, payoutID, domain.FailureProviderError, cause.Error()); err != nil && !errors.Is(err, domain.ErrInvalidPayoutState) {
		s.log.ErrorContext(ctx, "payout fail after exhaustion failed", logx.KeyOp, "Payout.SendExhausted", KeyPayoutID, payoutID,
			logx.KeyError, err.Error())
	}
}
