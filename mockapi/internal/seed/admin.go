package seed

import (
	"time"

	"github.com/google/uuid"

	"maxim/mockapi/internal/store"
)

// loadAdmin — данные разделов администратора: выписка и поступления, журнал проводок,
// объекты и отчёт ОРД прошлого месяца, журнал действий.
func loadAdmin(d *store.Data, now time.Time, ago func(h float64) time.Time) {
	// ------------------------------------------------ выписка и поступления
	day := now.In(store.MSK).AddDate(0, 0, -19)
	d.BankStatements = []*store.BankStatement{{ID: BankStatementFirst, UploadedBy: UserAdmin, AccountNumber: "40702810000000009999",
		DateFrom: day, DateTo: day, OpeningBalance: 20_000_000, ClosingBalance: 120_000_000, Incoming: 2, Matched: 1, Unmatched: 1,
		CreatedAt: ago(19 * 24)}}
	adv := OrgAdvertiser
	inv := InvoicePaid
	d.BankTxs = []*store.BankTx{
		{ID: ID(1311), StatementID: BankStatementFirst, OperationDate: day, Amount: 100_000_000, PayerName: "ООО «Тестовый рекламодатель»",
			PayerINN: "7700000001", PayerAccount: "40702810000000000001", Purpose: "Оплата по счёту MX-000101",
			MatchStatus: "matched", OrgID: &adv, InvoiceID: &inv, CreatedAt: ago(19 * 24)},
		{ID: BankTxUnmatched, StatementID: BankStatementFirst, OperationDate: day, Amount: 2_500_000, PayerName: "Иванов Иван Иванович",
			PayerINN: "770000000077", PayerAccount: "40817810000000000077", Purpose: "Оплата рекламы",
			MatchStatus: "unmatched", CreatedAt: ago(19 * 24)},
	}

	// ------------------------------------------------ журнал проводок (06 §2): активы = пассивы
	admin := UserAdmin
	bank := store.LedgerAccount{Kind: "bank"}
	deposit := store.LedgerAccount{Kind: "rocketwork_deposit"}
	balance := store.LedgerAccount{Kind: "advertiser_balance", OrgID: &adv}
	blg := OrgBlogger
	payable := store.LedgerAccount{Kind: "blogger_payable", OrgID: &blg}
	var paid int64
	for _, p := range d.Payouts {
		if p.ID == ID(1201) {
			paid = p.Amount
		}
	}
	po := ID(1201)
	d.AdminLedger = []*store.AdminLedgerTx{
		{ID: ID(1701), Kind: "deposit", Amount: 100_000_000, Debit: bank, Credit: balance, ActorType: "system",
			InvoiceID: &inv, CreatedAt: ago(19 * 24)},
		{ID: ID(1702), Kind: "deposit_topup", Amount: 5_000_000, Debit: deposit, Credit: bank, ActorType: "admin", ActorID: &admin,
			Reason: "Пополнение депозита по платёжке 12", CreatedAt: ago(18 * 24)},
		{ID: ID(1703), Kind: "settlement", Amount: paid, Debit: balance, Credit: payable, ActorType: "system", CreatedAt: ago(70)},
		{ID: ID(1704), Kind: "payout", Amount: paid, Debit: payable, Credit: deposit, ActorType: "system", PayoutID: &po, CreatedAt: ago(60)},
	}

	// ------------------------------------------------ объекты ОРД
	d.OrdObjects = []*store.OrdObject{
		{Type: "organization", ID: OrgAdvertiser, Title: "ООО «Тестовый рекламодатель»", ExternalID: OrgAdvertiser.String(),
			Status: "synced", OrdStatus: "success", Attempts: 1, UpdatedAt: ago(39 * 24),
			Request: map[string]any{"inn": "77****0001", "type": "ul"}, Response: map[string]any{"status": "success"}},
		{Type: "creative", ID: ID(751), Title: "Заказ № 1001", ExternalID: ID(751).String(), Status: "synced", OrdStatus: "success",
			Erid: "MockA1B2C3D4", Attempts: 1, UpdatedAt: ago(29), Request: map[string]any{"text_length": 120}, Response: map[string]any{"erid": "MockA1B2C3D4"}},
		{Type: "contract", ID: OrdContractError, Title: "Договор с блогером: Образцов Пётр Петрович", ExternalID: OrdContractError.String(),
			Status: "error", OrdStatus: "error", Error: "Неверный ИНН контрагента", Attempts: 3, UpdatedAt: ago(48),
			Request: map[string]any{"type": "service", "contractorInn": "77****0004"}, Response: map[string]any{"status": "error", "message": "invalid inn"}},
	}

	// ------------------------------------------------ отчёт ОРД за прошлый месяц
	prev := now.In(store.MSK).AddDate(0, -1, 0)
	start := time.Date(prev.Year(), prev.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)
	built := time.Date(now.Year(), now.Month(), 1, 0, 5, 0, 0, store.MSK).UTC()
	d.OrdReports = []*store.OrdReport{{
		Period: prev.Format("2006-01"), BuiltAt: &built,
		Statistics: []*store.OrdStatisticsRow{
			{ID: ID(1501), PlacementID: PlacementPaid, CreativeID: ID(752), Erid: "MockE5F6A7B8", ChannelID: ID(102),
				Impressions: 1900, Amount: 230_000, ShownFrom: start.AddDate(0, 0, 9), ShownTo: start.AddDate(0, 0, 10), Status: "synced", Attempts: 1},
			{ID: ID(1502), PlacementID: PlacementRemoved, CreativeID: ID(752), Erid: "MockE5F6A7B8", ChannelID: ID(101),
				Impressions: 3100, Amount: 345_000, ShownFrom: start.AddDate(0, 0, 14), ShownTo: end, Status: "error",
				Error: "Площадка не зарегистрирована в ОРД", Attempts: 2},
		},
		Invoices: []*store.OrdInvoiceRow{
			{ID: ID(1511), ContractKind: "initial", OrgID: OrgAdvertiser, Number: "MX-" + prev.Format("200601") + "-1",
				InvoiceDate: end, Amount: 575_000, ItemsCount: 2, Status: "pending"},
			{ID: ID(1512), ContractKind: "publisher", OrgID: OrgBlogger, Number: "MX-" + prev.Format("200601") + "-2",
				InvoiceDate: end, Amount: 500_000, ItemsCount: 2, Status: "pending"},
		},
	}}

	// ------------------------------------------------ журнал действий
	ch := ID(101)
	d.Audit = []*store.AuditEntry{
		{ID: ID(1801), ActorType: "admin", ActorID: &admin, OrgID: ptr(OrgBlogger), Action: "channel.approved", ObjectType: "channel",
			ObjectID: ch.String(), Data: map[string]any{}, CreatedAt: ago(200)},
		{ID: ID(1802), ActorType: "admin", ActorID: &admin, Action: "ledger.deposit_topup", ObjectType: "ledger_transaction",
			ObjectID: ID(1702).String(), Reason: "Пополнение депозита по платёжке 12", Data: map[string]any{"amount": 5_000_000}, CreatedAt: ago(18 * 24)},
		{ID: ID(1803), ActorType: "advertiser", ActorID: ptr(uuid.UUID(UserAdvertiser)), OrgID: &adv, Action: "placement.claim_created",
			ObjectType: "placement", ObjectID: PlacementDisputed.String(), Data: map[string]any{"reason": "removed_early"}, CreatedAt: ago(90)},
	}
}
