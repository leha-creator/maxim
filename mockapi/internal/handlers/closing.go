package handlers

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"maxim/mockapi/internal/api"
	"maxim/mockapi/internal/store"
)

func closingAct(a *store.ClosingAct) api.ClosingAct {
	return api.ClosingAct{Id: a.ID, Number: a.Number, Period: a.Period, PlacementsAmount: a.PlacementsAmount,
		FeeAmount: a.FeeAmount, CreatedAt: a.CreatedAt,
		ReportUrl: fmt.Sprintf("/api/v1/closing-acts/%s/report", a.ID),
		ActUrl:    fmt.Sprintf("/api/v1/closing-acts/%s/act", a.ID)}
}

func (h *Handlers) ListClosingActs(ctx context.Context, req api.ListClosingActsRequestObject) (api.ListClosingActsResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	_, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	var mine []*store.ClosingAct
	for _, a := range tx.D.ClosingActs {
		if a.OrgID == o.ID {
			mine = append(mine, a)
		}
	}
	mine = newestFirst(mine, func(a *store.ClosingAct) time.Time { return a.CreatedAt })
	pg, next := page(mine, req.Params.Limit, req.Params.Cursor)
	items := []api.ClosingAct{}
	for _, a := range pg {
		items = append(items, closingAct(a))
	}
	return api.ListClosingActs200JSONResponse{Items: items, NextCursor: next}, nil
}

func (h *Handlers) GetClosingActPdf(ctx context.Context, req api.GetClosingActPdfRequestObject) (api.GetClosingActPdfResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	_, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	a, err := tx.ClosingAct(o.ID, req.Id)
	if err != nil {
		return nil, err
	}
	// В PDF только латиница: шрифт Helvetica без кириллицы.
	pdf := invoicePDF(fmt.Sprintf("TEST %s FOR %s. Placements: %d.%02d RUB, fee: %d.%02d RUB. Not a real document.",
		req.Document, a.Period, a.PlacementsAmount/100, a.PlacementsAmount%100, a.FeeAmount/100, a.FeeAmount%100))
	return api.GetClosingActPdf200ApplicationpdfResponse{Body: bytes.NewReader(pdf), ContentLength: int64(len(pdf))}, nil
}
