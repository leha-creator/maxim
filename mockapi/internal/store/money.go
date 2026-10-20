package store

import (
	"fmt"

	"github.com/google/uuid"
)

// FeeRateBP — ставка комиссии платформы в базисных пунктах. // A-30, Q-22
const FeeRateBP = 1500

// TotalFor возвращает комиссию и итог для рекламодателя по цене блогера.
// Округление комиссии — вниз до копейки (README «Деньги»).
func TotalFor(bloggerAmount int64) (fee, total int64) {
	fee = bloggerAmount * FeeRateBP / 10000
	return fee, bloggerAmount + fee
}

// Balance возвращает доступный остаток и сумму активных удержаний организации.
func (tx *Tx) Balance(orgID uuid.UUID) (available, held int64) {
	for _, e := range tx.D.Ledger {
		if e.OrgID == orgID {
			available += e.Amount
		}
	}
	for _, p := range tx.D.Placements {
		if p.HoldActive {
			if o, err := tx.Order(p.OrderID); err == nil && o.OrgID == orgID {
				held += p.TotalAmount
			}
		}
	}
	return available, held
}

func (tx *Tx) addEntry(orgID uuid.UUID, kind string, amount int64, desc string, orderID, placementID, invoiceID *uuid.UUID) {
	tx.D.Ledger = append(tx.D.Ledger, &LedgerEntry{
		ID: NewID(), OrgID: orgID, Kind: kind, Amount: amount, Description: desc,
		OrderID: orderID, PlacementID: placementID, InvoiceID: invoiceID, CreatedAt: tx.Now,
	})
	tx.log.Info("balance entry created", "op", "Money.Entry", "organization_id", orgID,
		"kind", kind, "amount", amount)
}

// Deposit зачисляет оплату счёта на баланс.
func (tx *Tx) Deposit(inv *Invoice) {
	id := inv.ID
	tx.addEntry(inv.OrgID, "deposit", inv.Amount, fmt.Sprintf("Пополнение по счёту %s", inv.Number), nil, nil, &id)
}

// hold удерживает итог размещения с баланса рекламодателя.
func (tx *Tx) hold(o *Order, p *Placement, channelTitle string) {
	oid, pid := o.ID, p.ID
	p.HoldActive = true
	tx.addEntry(o.OrgID, "hold", -p.TotalAmount,
		fmt.Sprintf("Удержание: заказ %d, канал «%s»", o.Number, channelTitle), &oid, &pid, nil)
}

// releaseHold снимает удержание целиком и возвращает деньги на баланс.
func (tx *Tx) releaseHold(p *Placement) {
	if !p.HoldActive {
		return
	}
	o, err := tx.Order(p.OrderID)
	if err != nil {
		return
	}
	oid, pid := o.ID, p.ID
	p.HoldActive = false
	tx.addEntry(o.OrgID, "hold_release", p.TotalAmount,
		fmt.Sprintf("Снятие удержания: заказ %d", o.Number), &oid, &pid, nil)
}

// distributeFull распределяет удержание при приёмке с долей 10000 (P-12, P-13)
// и создаёт выплату блогеру.
func (tx *Tx) distributeFull(p *Placement) {
	share := 10000
	payout, fee, refund := p.BloggerAmount, p.FeeAmount, int64(0)
	p.PerformanceShare = &share
	p.PayoutAmount, p.FeeCharged, p.RefundAmount = &payout, &fee, &refund
	p.HoldActive = false
	tx.D.Payouts = append(tx.D.Payouts, &Payout{
		ID: NewID(), PlacementID: p.ID, OrgID: p.BloggerOrgID, Amount: payout,
		Status: "created", Attempt: 1, CreatedAt: tx.Now,
	})
	tx.log.Info("payout created", "op", "Money.Distribute", "placement_id", p.ID, "amount", payout)
}
