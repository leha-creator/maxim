package domain

import (
	"time"

	"github.com/google/uuid"
)

// Приёмка и споры (этап «Приёмка, споры и возвраты», 03 §4): P-12, P-13, P-14 рекламодателем и
// администратором, P-15. Деньги (Distribute, Holds.Settle, выплата) — в app, в той же транзакции.

// AcceptWork — работа принята: P-12 рекламодателем (S-08) или P-13 по T-05 (S-24). Окно претензий
// закрывается: T-05 отменяется. Уведомлений нет — блогер узнаёт о приёмке из payout.sent (R-14).
func (p *Placement) AcceptWork(code TransitionCode, actor ActorType, userID *uuid.UUID, now time.Time) (StateChange, error) {
	reason := ""
	if code == P13AutoAccept {
		reason = ReasonAutoAcceptance
	}
	change, err := p.Apply(code, actor, reason, now)
	if err != nil {
		return StateChange{}, err
	}
	change.UserID = userID
	change.CancelJobs = []string{TimerKey(p.ID, TimerClaim)}
	return change, nil
}

// FileClaim — P-14 по претензии рекламодателя (S-08): окно T-05 должно быть открыто (R-2), T-05
// отменяется, ставится T-06, уведомляются обе стороны и администратор.
func (p *Placement) FileClaim(userID uuid.UUID, reason ClaimReason, now time.Time, r ControlRules) (StateChange, error) {
	if p.Status == StatusRemoved && !ClaimWindowOpen(p, now) {
		return StateChange{}, &TransitionError{Code: P14OpenDispute, Status: p.Status, Reason: ReasonClaimWindowClosed}
	}
	change, err := p.Apply(P14OpenDispute, ActorAdvertiser, "claim_"+string(reason), now)
	if err != nil {
		return StateChange{}, err
	}
	change.UserID = &userID
	p.disputeOpened(&change, now, r)
	return change, nil
}

// OpenDisputeByAdmin — P-14 администратором (admin_open_dispute, R-12): причина admin, основание —
// в reason_text истории.
func (p *Placement) OpenDisputeByAdmin(adminID uuid.UUID, text string, now time.Time, r ControlRules) (StateChange, error) {
	change, err := p.Apply(P14OpenDispute, ActorAdmin, DisputeReasonAdmin, now)
	if err != nil {
		return StateChange{}, err
	}
	change.UserID, change.ReasonText = &adminID, text
	p.disputeOpened(&change, now, r)
	return change, nil
}

// disputeOpened — общие действия P-14 (03 §4): отменить T-05, поставить T-06, уведомить обоих и
// администратора.
func (p *Placement) disputeOpened(change *StateChange, now time.Time, r ControlRules) {
	change.CancelJobs = []string{TimerKey(p.ID, TimerClaim)}
	change.Jobs = []Job{TimerJob(p.ID, TimerDispute, now.Add(r.DisputeDeadline), StatusDisputed)}
	change.Notices = append(p.notifyBoth(NoticeDisputed), NoticeTo{Template: NoticeDisputed, Role: ActorAdmin})
}

// ResolveDispute — P-15 (S-18): accepted (full_payout, proportional) или refunded (full_refund) по
// распределению d. T-06 отменяется, обе стороны получают placement.dispute_resolved с исходом и
// суммами (R-14). Распределение записывается в размещение (SetDistribution).
func (p *Placement) ResolveDispute(res Resolution, adminID uuid.UUID, d Distribution, now time.Time) (StateChange, error) {
	change, err := p.ApplyTo(P15ResolveDispute, res.Status(), ActorAdmin, string(res), now)
	if err != nil {
		return StateChange{}, err
	}
	p.SetDistribution(d)
	change.UserID = &adminID
	change.CancelJobs = []string{TimerKey(p.ID, TimerDispute)}
	data := map[string]any{
		"resolution": string(res), "performance_share": d.PerformanceShare,
		"refund_amount": int64(d.RefundAmount), "payout_amount": int64(d.PayoutAmount),
	}
	for _, n := range p.notifyBoth(NoticeDisputeResolved) {
		n.Data = data
		change.Notices = append(change.Notices, n)
	}
	return change, nil
}

// SetDistribution записывает суммы распределения в размещение (03 §7, I-12).
func (p *Placement) SetDistribution(d Distribution) {
	share, payout, fee, refund := d.PerformanceShare, d.PayoutAmount, d.FeeCharged, d.RefundAmount
	p.PerformanceShare, p.PayoutAmount, p.FeeCharged, p.RefundAmount = &share, &payout, &fee, &refund
}

// CheckSettleable — I-02: в accepted размещение попадает только при снятой публикации
// (publication.removed_at заполнено). Публикации нет — тоже отказ: принимать нечего.
func (p *Placement) CheckSettleable(code TransitionCode, pub *Publication) error {
	if pub == nil || pub.Live() {
		return &TransitionError{Code: code, Status: p.Status, Reason: ReasonPublicationLive}
	}
	return nil
}

// ClaimSuggestedShare — предложение доли по претензии (R-3 этапа приёмки, 03 §7): removed_early — по
// снятой публикации, top_violated — параметр, остальные — нет.
func ClaimSuggestedShare(reason ClaimReason, pub Publication, removeAt time.Time, r ControlRules) *int {
	switch reason {
	case ClaimRemovedEarly:
		if pub.RemovedAt == nil {
			return nil
		}
		return SuggestedShare(ViolationRemovedEarly, pub.PublishedAt, *pub.RemovedAt, removeAt, r)
	case ClaimTopViolated:
		return SuggestedShare(ViolationTopViolated, pub.PublishedAt, pub.PublishedAt, removeAt, r)
	}
	return nil
}
