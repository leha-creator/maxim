package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func mskAt(y int, m time.Month, d, h int) time.Time { return time.Date(y, m, d, h, 0, 0, 0, msk) }

func TestPeriod(t *testing.T) {
	p, err := ParsePeriod("2026-09")
	if err != nil || p.String() != "2026-09" || !p.Start().Equal(mskAt(2026, 9, 1, 0)) || !p.End().Equal(mskAt(2026, 10, 1, 0)) {
		t.Fatalf("period: %+v %v", p, err)
	}
	for _, bad := range []string{"2026-9", "2026-13", "09-2026", ""} {
		if _, err := ParsePeriod(bad); err == nil {
			t.Errorf("ParsePeriod(%q) accepted", bad)
		}
	}
	if got := p.DueDate().Format(time.DateOnly); got != "2026-10-30" {
		t.Errorf("due date %s", got)
	}
	if got := (Period{2027, time.January}).DueDate().Format(time.DateOnly); got != "2027-02-28" {
		t.Errorf("due date in February %s", got)
	}
	if got := p.LastDay().Format(time.DateOnly); got != "2026-09-30" {
		t.Errorf("last day %s", got)
	}
	// Граница суток по Москве: 30 сентября 22:00 UTC — уже 1 октября.
	if got := PeriodOf(time.Date(2026, 9, 30, 22, 0, 0, 0, time.UTC)); got.String() != "2026-10" {
		t.Errorf("period of MSK midnight %s", got)
	}
	if p.Closed(mskAt(2026, 9, 30, 23)) || !p.Closed(mskAt(2026, 10, 1, 0)) {
		t.Error("closed")
	}
	if p.Prev().String() != "2026-08" || p.Next().String() != "2026-10" {
		t.Errorf("prev/next %s %s", p.Prev(), p.Next())
	}
}

func TestBuildStatistics(t *testing.T) {
	sep := Period{2026, time.September}
	cr, pl := uuid.New(), uuid.New()
	initial, publisher := uuid.New(), uuid.New()
	base := ReportPlacement{PlacementID: uuid.New(), CreativeID: cr, PlatformID: pl, InitialContractID: initial,
		PublisherContractID: publisher, AdvertiserAmount: 345000, BloggerAmount: 300000}
	date := func(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }

	cases := []struct {
		name   string
		in     func() ReportPlacement
		period Period
		want   *StatisticsLine // nil — строки нет
	}{
		{"inside month", func() ReportPlacement {
			p := base
			p.PublishedAt, p.RemovedAt, p.FinalViews = mskAt(2026, 9, 10, 12), mskAt(2026, 9, 12, 12), 1200
			return p
		}, sep, &StatisticsLine{Impressions: 1200, Amount: 345000, ShownFrom: date("2026-09-10"), ShownTo: date("2026-09-12")}},
		{"crosses border, first month", func() ReportPlacement {
			p := base
			p.PublishedAt, p.RemovedAt, p.FinalViews = mskAt(2026, 9, 29, 12), mskAt(2026, 10, 2, 12), 2000
			p.ViewsAtEnd, p.HasEnd = 800, true
			return p
		}, sep, &StatisticsLine{Impressions: 800, Amount: 0, ShownFrom: date("2026-09-29"), ShownTo: date("2026-09-30")}},
		{"crosses border, second month", func() ReportPlacement {
			p := base
			p.PublishedAt, p.RemovedAt, p.FinalViews = mskAt(2026, 9, 29, 12), mskAt(2026, 10, 2, 12), 2000
			p.ViewsAtStart, p.HasStart = 800, true
			return p
		}, Period{2026, time.October}, &StatisticsLine{Impressions: 1200, Amount: 345000, ShownFrom: date("2026-10-01"), ShownTo: date("2026-10-02")}},
		{"snapshot above final", func() ReportPlacement {
			p := base
			p.PublishedAt, p.RemovedAt, p.FinalViews = mskAt(2026, 8, 30, 12), mskAt(2026, 9, 2, 12), 500
			p.ViewsAtStart, p.HasStart = 700, true
			return p
		}, sep, &StatisticsLine{Impressions: 0, Amount: 345000, ShownFrom: date("2026-09-01"), ShownTo: date("2026-09-02")}},
		{"removed before month", func() ReportPlacement {
			p := base
			p.PublishedAt, p.RemovedAt = mskAt(2026, 8, 1, 12), mskAt(2026, 8, 31, 23)
			return p
		}, sep, nil},
		{"published after month", func() ReportPlacement {
			p := base
			p.PublishedAt = mskAt(2026, 10, 1, 0)
			return p
		}, sep, nil},
		{"MSK day border", func() ReportPlacement {
			p := base
			// 31 августа 21:30 UTC — 1 сентября 00:30 по Москве.
			p.PublishedAt, p.RemovedAt, p.FinalViews = time.Date(2026, 8, 31, 21, 30, 0, 0, time.UTC), mskAt(2026, 9, 1, 5), 10
			return p
		}, sep, &StatisticsLine{Impressions: 10, Amount: 345000, ShownFrom: date("2026-09-01"), ShownTo: date("2026-09-01")}},
	}
	for _, tc := range cases {
		got := BuildStatistics([]ReportPlacement{tc.in()}, tc.period)
		if tc.want == nil {
			if len(got) != 0 {
				t.Errorf("%s: want no line, got %+v", tc.name, got)
			}
			continue
		}
		if len(got) != 1 {
			t.Fatalf("%s: lines %+v", tc.name, got)
		}
		g := got[0]
		if g.Impressions != tc.want.Impressions || g.Amount != tc.want.Amount || !g.ShownFrom.Equal(tc.want.ShownFrom) ||
			!g.ShownTo.Equal(tc.want.ShownTo) || g.CreativeID != cr || g.PlatformID != pl {
			t.Errorf("%s: got %+v, want %+v", tc.name, g, *tc.want)
		}
	}

	// Две публикации одной пары — одна строка с суммой.
	a, b := base, base
	a.PublishedAt, a.RemovedAt, a.FinalViews = mskAt(2026, 9, 3, 10), mskAt(2026, 9, 4, 10), 100
	b.PlacementID = uuid.New()
	b.PublishedAt, b.RemovedAt, b.FinalViews = mskAt(2026, 9, 20, 10), mskAt(2026, 9, 21, 10), 50
	lines := BuildStatistics([]ReportPlacement{a, b}, sep)
	if len(lines) != 1 || lines[0].Impressions != 150 || lines[0].Amount != 690000 || len(lines[0].Placements) != 2 ||
		lines[0].ShownTo.Format(time.DateOnly) != "2026-09-21" {
		t.Fatalf("two publications: %+v", lines)
	}

	// Акты: рекламодателя и блогера, разбивка по паре.
	inv := BuildInvoices(lines)
	if len(inv) != 2 || inv[0].Kind != ContractInitial || inv[0].ContractID != initial || inv[0].Amount != 690000 ||
		inv[1].Kind != ContractPublisher || inv[1].Amount != 600000 || len(inv[1].Items) != 1 || inv[1].Items[0].Amount != 600000 {
		t.Fatalf("invoices: %+v", inv)
	}
	// Нулевые суммы акта не дают.
	zero := a
	zero.AdvertiserAmount, zero.BloggerAmount = 0, 0
	if inv := BuildInvoices(BuildStatistics([]ReportPlacement{zero}, sep)); len(inv) != 0 {
		t.Fatalf("zero invoices: %+v", inv)
	}
	if n := InvoiceNumber("MX", sep, 7); n != "MX-202609-7" {
		t.Errorf("invoice number %s", n)
	}
	if Rubles(345001) != "3450.01" || Rubles(0) != "0.00" {
		t.Errorf("rubles %s %s", Rubles(345001), Rubles(0))
	}
}
