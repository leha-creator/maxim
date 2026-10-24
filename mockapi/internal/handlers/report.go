package handlers

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"maxim/mockapi/internal/api"
	"maxim/mockapi/internal/store"
)

// Отчёт по кампании (S-09) по тем же правилам, что у бэкенда (placement/domain.BuildReport).

func (h *Handlers) GetOrderReport(ctx context.Context, req api.GetOrderReportRequestObject) (api.GetOrderReportResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	_, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	x, err := tx.OrgOrder(o.ID, req.Id)
	if err != nil {
		return nil, err
	}
	out := orderReport(tx, x)
	h.logger(ctx).Debug("[FIX] order report built", "op", "Order.Report", "order_id", x.ID, "rows", len(out.Rows),
		"spent_amount", out.Totals.SpentAmount, "refunded_amount", out.Totals.RefundedAmount)
	return api.GetOrderReport200JSONResponse(out), nil
}

func (h *Handlers) GetOrderReportCsv(ctx context.Context, req api.GetOrderReportCsvRequestObject) (api.GetOrderReportCsvResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	_, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	x, err := tx.OrgOrder(o.ID, req.Id)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := writeReportCSV(&b, orderReport(tx, x)); err != nil {
		return nil, err
	}
	return reportCSV{name: fmt.Sprintf("order-%d-report.csv", x.Number), body: b.Bytes()}, nil
}

type reportCSV struct {
	name string
	body []byte
}

func (r reportCSV) VisitGetOrderReportCsvResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, r.name))
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(r.body)
	return err
}

// viewCost — цена просмотра с округлением вниз; без просмотров — нет (D-28).
func viewCost(spent int64, views *int) *int64 {
	if views == nil || *views <= 0 || spent <= 0 {
		return nil
	}
	v := spent / int64(*views)
	return &v
}

func orderReport(tx *store.Tx, x *store.Order) api.OrderReport {
	s := orderSummary(tx, x)
	campaign := fmt.Sprintf("order_%d", x.Number) // A-21
	links := store.BodyLinks(x.CurrentVersion().Body)
	out := api.OrderReport{OrderId: x.ID, OrderNumber: x.Number, CampaignName: s.CampaignName, Rows: []api.OrderReportRow{},
		Links: links, Utm: api.OrderReportUtm{Source: "max", Medium: "channel_ad", Campaign: campaign}}
	for _, p := range tx.OrderPlacements(x.ID) {
		ch, _ := tx.Channel(p.ChannelID)
		row := api.OrderReportRow{PlacementId: p.ID, Channel: channelRef(ch), Status: api.PlacementStatus(p.Status),
			PublishedAt: p.PublishedAt, RemovedAt: p.RemovedAt, Views: p.Views, PublicationUrl: strPtr(p.PublicationURL),
			UtmCampaign: ptr(campaign)}
		// Итоги — сумма строк: фронт больше не считает сумму строки сам.
		switch {
		case p.Status == "accepted" || p.Status == "paid":
			refund := int64(0)
			if p.RefundAmount != nil {
				refund = *p.RefundAmount
			}
			row.SpentAmount, row.RefundedAmount = p.TotalAmount-refund, refund
		case p.HoldActive:
			row.HeldAmount = p.TotalAmount // в работе или в споре
		case store.IsTerminal(p.Status):
			row.RefundedAmount = p.TotalAmount
		}
		row.CpvAmount = viewCost(row.SpentAmount, row.Views)
		out.Rows = append(out.Rows, row)
		out.Totals.Placements++
		if p.PublishedAt != nil {
			out.Totals.Published++
		}
		if p.Views != nil {
			out.Totals.Views += *p.Views
		}
		out.Totals.SpentAmount += row.SpentAmount
		out.Totals.RefundedAmount += row.RefundedAmount
		out.Totals.HeldAmount += row.HeldAmount
	}
	views := out.Totals.Views
	out.Totals.CpvAmount = viewCost(out.Totals.SpentAmount, &views)
	return out
}

var msk = time.FixedZone("MSK", 3*60*60)

// writeReportCSV — тот же формат, что у бэкенда: BOM, «;», рубли с запятой, время по Москве.
func writeReportCSV(w io.Writer, r api.OrderReport) error {
	if _, err := io.WriteString(w, "\uFEFF"); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	cw.Comma, cw.UseCRLF = ';', true
	rub := func(k int64) string { return fmt.Sprintf("%d,%02d", k/100, k%100) }
	rubPtr := func(k *int64) string {
		if k == nil {
			return ""
		}
		return rub(*k)
	}
	at := func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return t.In(msk).Format("02.01.2006 15:04")
	}
	num := func(v *int) string {
		if v == nil {
			return ""
		}
		return strconv.Itoa(*v)
	}
	_ = cw.Write([]string{"Канал", "Ссылка на канал", "Состояние", "Опубликовано (МСК)", "Снято (МСК)", "Просмотры",
		"CPV, ₽", "Списано, ₽", "Возвращено, ₽", "Удержано, ₽", "Ссылка на пост", "utm_campaign"})
	for _, row := range r.Rows {
		link, url := "", ""
		if row.Channel.Link != nil {
			link = *row.Channel.Link
		}
		if row.PublicationUrl != nil {
			url = *row.PublicationUrl
		}
		_ = cw.Write([]string{row.Channel.Title, link, string(row.Status), at(row.PublishedAt), at(row.RemovedAt), num(row.Views),
			rubPtr(row.CpvAmount), rub(row.SpentAmount), rub(row.RefundedAmount), rub(row.HeldAmount), url, r.Utm.Campaign})
	}
	t := r.Totals
	views := t.Views
	_ = cw.Write([]string{"Итого", "", fmt.Sprintf("Размещений: %d, опубликовано: %d", t.Placements, t.Published), "", "",
		num(&views), rubPtr(t.CpvAmount), rub(t.SpentAmount), rub(t.RefundedAmount), rub(t.HeldAmount), "", r.Utm.Campaign})
	cw.Flush()
	return cw.Error()
}
