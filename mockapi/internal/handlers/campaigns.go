package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"maxim/mockapi/internal/api"
	"maxim/mockapi/internal/store"
)

func date(t *time.Time) *openapi_types.Date {
	if t == nil {
		return nil
	}
	return &openapi_types.Date{Time: *t}
}

func campaign(c *store.Campaign) api.Campaign {
	return api.Campaign{
		Id: c.ID, Name: c.Name, Status: api.CampaignStatus(c.Status), DateFrom: date(c.DateFrom), DateTo: date(c.DateTo),
		TimeFrom: c.TimeFrom, TimeTo: c.TimeTo, ExtraRequirements: c.ExtraRequirements, OrderId: c.OrderID,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

// Пределы формы кампании (A-39, R-17) — как в бэкенде.
const (
	maxCampaignPeriodDays   = 14
	minCampaignWindowMinute = 60
)

// applyCampaignInput переносит поля в кампанию и проверяет период (A-39).
// Отказ — 422 с details.reason; кампания при отказе не меняется.
func (h *Handlers) applyCampaignInput(ctx context.Context, tx *store.Tx, c *store.Campaign, in *api.CampaignInput) error {
	periodChanged := in.DateFrom != nil || in.DateTo != nil || in.TimeFrom != nil || in.TimeTo != nil
	if periodChanged && c.Status != "draft" {
		return store.Conflict("conflict", "campaign period can be changed only in draft", map[string]any{"status": c.Status})
	}
	next := *c
	if in.Name != nil {
		next.Name = strings.TrimSpace(*in.Name)
		if next.Name == "" {
			return store.Invalid("name", "required", "name is required")
		}
	}
	if in.ExtraRequirements != nil {
		next.ExtraRequirements = in.ExtraRequirements
	}
	if in.DateFrom != nil {
		next.DateFrom = &in.DateFrom.Time
	}
	if in.DateTo != nil {
		next.DateTo = &in.DateTo.Time
	}
	if in.TimeFrom != nil {
		next.TimeFrom = in.TimeFrom
	}
	if in.TimeTo != nil {
		next.TimeTo = in.TimeTo
	}
	if err := checkCampaignPeriod(tx, &next, in.DateFrom != nil || in.DateTo != nil); err != nil {
		h.logger(ctx).Warn("[FIX] campaign input rejected", "op", "Campaign.Apply", "campaign_id", c.ID,
			"field", err.Details["field"], "reason", err.Details["reason"])
		return err
	}
	next.UpdatedAt = tx.Now
	*c = next
	return nil
}

// checkCampaignPeriod — правила периода дат и окна времени (A-39, R-17).
// «Прошлое» проверяется, только когда меняются даты.
func checkCampaignPeriod(tx *store.Tx, c *store.Campaign, datesChanged bool) *store.Error {
	if c.DateFrom != nil && c.DateTo != nil {
		if c.DateFrom.After(*c.DateTo) {
			return store.Invalid("date_from", "period_order", "date_from is after date_to")
		}
		if days := int(c.DateTo.Sub(*c.DateFrom).Hours()/24) + 1; days > maxCampaignPeriodDays {
			return store.Invalid("date_to", "period_too_long", fmt.Sprintf("period is longer than %d days", maxCampaignPeriodDays))
		}
	}
	if datesChanged {
		n := tx.Now.In(store.MSK)
		today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
		if (c.DateFrom != nil && c.DateFrom.Before(today)) || (c.DateFrom == nil && c.DateTo != nil && c.DateTo.Before(today)) {
			return store.Invalid("date_from", "period_in_past", "date_from is in the past")
		}
	}
	if (c.TimeFrom == nil) != (c.TimeTo == nil) {
		return store.Invalid("time_from", "time_pair", "time_from and time_to must be set together")
	}
	if c.TimeFrom != nil {
		from, errFrom := time.Parse("15:04", *c.TimeFrom)
		to, errTo := time.Parse("15:04", *c.TimeTo)
		if errFrom != nil || errTo != nil || to.Sub(from) < minCampaignWindowMinute*time.Minute {
			return store.Invalid("time_from", "time_window", "time window is shorter than 1 hour")
		}
	}
	return nil
}

func (h *Handlers) ListCampaigns(ctx context.Context, req api.ListCampaignsRequestObject) (api.ListCampaignsResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	_, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	var mine []*store.Campaign
	for _, c := range tx.D.Campaigns {
		if c.OrgID == o.ID && (req.Params.Status == nil || string(*req.Params.Status) == c.Status) {
			mine = append(mine, c)
		}
	}
	mine = newestFirst(mine, func(c *store.Campaign) time.Time { return c.CreatedAt })
	pg, next, total := page(mine, req.Params.Limit, req.Params.Cursor)
	items := []api.Campaign{}
	for _, c := range pg {
		items = append(items, campaign(c))
	}
	return api.ListCampaigns200JSONResponse{Items: items, NextCursor: next, Total: total}, nil
}

func (h *Handlers) CreateCampaign(ctx context.Context, req api.CreateCampaignRequestObject) (api.CreateCampaignResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	_, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	if req.Body.Name == nil || *req.Body.Name == "" {
		return nil, store.Invalid("name", "required", "name is required")
	}
	c := &store.Campaign{ID: store.NewID(), OrgID: o.ID, Status: "draft", CreatedAt: tx.Now,
		Post: store.PostDraft{Doc: api.EditorDoc{Type: "doc", Content: []api.EditorBlock{}}, MediaIDs: []api.Uuid{},
			TokenCarrier: "text", TokenPosition: "below", UpdatedAt: tx.Now}}
	if err := h.applyCampaignInput(ctx, tx, c, req.Body); err != nil {
		return nil, err
	}
	tx.D.Campaigns = append(tx.D.Campaigns, c)
	h.logger(ctx).Info("campaign created", "op", "Campaign.Create", "organization_id", o.ID, "campaign_id", c.ID)
	return api.CreateCampaign201JSONResponse(campaign(c)), nil
}

func (h *Handlers) GetCampaign(ctx context.Context, req api.GetCampaignRequestObject) (api.GetCampaignResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	_, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	c, err := tx.Campaign(o.ID, req.Id)
	if err != nil {
		return nil, err
	}
	return api.GetCampaign200JSONResponse(campaign(c)), nil
}

func (h *Handlers) UpdateCampaign(ctx context.Context, req api.UpdateCampaignRequestObject) (api.UpdateCampaignResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	_, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	c, err := tx.Campaign(o.ID, req.Id)
	if err != nil {
		return nil, err
	}
	if err := h.applyCampaignInput(ctx, tx, c, req.Body); err != nil {
		return nil, err
	}
	h.logger(ctx).Info("campaign updated", "op", "Campaign.Update", "campaign_id", c.ID)
	return api.UpdateCampaign200JSONResponse(campaign(c)), nil
}

// ---------------------------------------------------------------- пост

func postDraft(tx *store.Tx, c *store.Campaign) api.PostDraft {
	media := []api.MediaFile{}
	for _, id := range c.Post.MediaIDs {
		if m, err := tx.Media(c.OrgID, id); err == nil {
			media = append(media, mediaFile(m))
		}
	}
	return api.PostDraft{CampaignId: c.ID, ContentDoc: c.Post.Doc, Media: media,
		TokenCarrier: api.TokenCarrier(c.Post.TokenCarrier), TokenPosition: api.TokenPosition(c.Post.TokenPosition),
		UpdatedAt: c.Post.UpdatedAt}
}

func (h *Handlers) GetCampaignPost(ctx context.Context, req api.GetCampaignPostRequestObject) (api.GetCampaignPostResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	_, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	c, err := tx.Campaign(o.ID, req.Id)
	if err != nil {
		return nil, err
	}
	return api.GetCampaignPost200JSONResponse(postDraft(tx, c)), nil
}

func (h *Handlers) SaveCampaignPost(ctx context.Context, req api.SaveCampaignPostRequestObject) (api.SaveCampaignPostResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	_, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	c, err := tx.Campaign(o.ID, req.Id)
	if err != nil {
		return nil, err
	}
	b := req.Body
	for _, id := range b.MediaFileIds {
		m, err := tx.Media(o.ID, id)
		if err != nil || m.DeletedAt != nil {
			return nil, store.Validation("media_file_ids", "media file not found: "+id.String())
		}
	}
	carrier := store.DefaultCarrier(len(b.MediaFileIds)) // D-30
	if b.TokenCarrier != nil {
		carrier = string(*b.TokenCarrier)
	}
	position := "below"
	if b.TokenPosition != nil {
		position = string(*b.TokenPosition)
	}
	c.Post = store.PostDraft{Doc: b.ContentDoc, MediaIDs: slices.Clone(b.MediaFileIds), TokenCarrier: carrier,
		TokenPosition: position, UpdatedAt: tx.Now}
	c.UpdatedAt = tx.Now
	h.logger(ctx).Info("post draft saved", "op", "Post.Save", "campaign_id", c.ID, "media", len(b.MediaFileIds),
		"token_carrier", carrier, "token_position", position)
	return api.SaveCampaignPost200JSONResponse(postDraft(tx, c)), nil
}

func (h *Handlers) PreviewCampaignPost(ctx context.Context, req api.PreviewCampaignPostRequestObject) (api.PreviewCampaignPostResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	_, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	c, err := tx.Campaign(o.ID, req.Id)
	if err != nil {
		return nil, err
	}
	body, problems := store.CheckDraft(c.Post)
	out := api.PreviewCampaignPost200JSONResponse{
		Body: body, BodyLength: len([]rune(body)), BodyLimit: store.BodyLimit,
		FinalTextSample: store.FinalText(body, c.Post.TokenCarrier, c.Post.TokenPosition, o.Name, o.INN, "ERID-EXAMPLE"),
		MediaUrls:       versionMediaURLs(tx, o.ID, c.Post.MediaIDs, c.Post.TokenCarrier),
		Problems:        []api.PostProblem{},
	}
	for _, p := range problems {
		out.Problems = append(out.Problems, api.PostProblem{Code: api.PostProblemCode(p.Code), Message: p.Message})
	}
	h.logger(ctx).Debug("post preview built", "op", "Post.Preview", "campaign_id", c.ID, "body_length", out.BodyLength, "problems", len(problems))
	return out, nil
}

func (h *Handlers) CheckoutCampaign(ctx context.Context, req api.CheckoutCampaignRequestObject) (api.CheckoutCampaignResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	u, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	c, err := tx.Campaign(o.ID, req.Id)
	if err != nil {
		return nil, err
	}
	ord, err := tx.Checkout(o, u.ID, c)
	if err != nil {
		return nil, err
	}
	return api.CheckoutCampaign201JSONResponse(order(tx, ord)), nil
}

// ---------------------------------------------------------------- медиатека

const maxUpload = 50 << 20 // 07 §4

func (h *Handlers) ListMedia(ctx context.Context, req api.ListMediaRequestObject) (api.ListMediaResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	_, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	var mine []*store.MediaFile
	for _, m := range tx.D.Media {
		if m.OrgID == o.ID && m.DeletedAt == nil {
			mine = append(mine, m)
		}
	}
	mine = newestFirst(mine, func(m *store.MediaFile) time.Time { return m.CreatedAt })
	pg, next, total := page(mine, req.Params.Limit, req.Params.Cursor)
	items := []api.MediaFile{}
	for _, m := range pg {
		items = append(items, mediaFile(m))
	}
	return api.ListMedia200JSONResponse{Items: items, NextCursor: next, Total: total}, nil
}

func (h *Handlers) UploadMedia(ctx context.Context, req api.UploadMediaRequestObject) (api.UploadMediaResponseObject, error) {
	// Файл читается до блокировки: загрузка может быть долгой.
	var data []byte
	var name string
	for {
		part, err := req.Body.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, store.Validation("file", "malformed multipart body")
		}
		if part.FormName() != "file" {
			continue
		}
		name = part.FileName()
		data, err = io.ReadAll(io.LimitReader(part, maxUpload+1))
		if err != nil {
			return nil, store.Validation("file", "cannot read file")
		}
	}
	if data == nil {
		return nil, store.Validation("file", "file part is missing")
	}
	if len(data) > maxUpload {
		return nil, store.Validation("file", "file is larger than 50 MB")
	}
	mime := http.DetectContentType(data) // тип — по содержимому, не по расширению
	if !slices.Contains([]string{"image/jpeg", "image/png", "image/webp"}, mime) {
		h.logger(ctx).Warn("upload rejected: unsupported type", "op", "Media.Upload", "mime", mime)
		return nil, store.Validation("file", "only JPEG, PNG and WebP are accepted")
	}
	w, hgt := 1080, 1080 // WebP стандартная библиотека не разбирает — размеры условные
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		w, hgt = cfg.Width, cfg.Height
	}
	if w > 7680 || hgt > 7680 {
		return nil, store.Validation("file", "image is larger than 7680×7680")
	}
	if name == "" {
		name = "upload"
	}

	tx, done := h.st.Begin(ctx)
	defer done()
	_, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	m := &store.MediaFile{ID: store.NewID(), OrgID: o.ID, Name: name, Mime: mime, SizeBytes: int64(len(data)),
		Width: w, Height: hgt, CreatedAt: tx.Now}
	tx.D.Media = append(tx.D.Media, m)
	h.logger(ctx).Info("media uploaded", "op", "Media.Upload", "organization_id", o.ID, "media_file_id", m.ID,
		"mime", mime, "size_bytes", m.SizeBytes)
	return api.UploadMedia201JSONResponse(mediaFile(m)), nil
}

func (h *Handlers) DeleteMedia(ctx context.Context, req api.DeleteMediaRequestObject) (api.DeleteMediaResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	_, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	m, err := tx.Media(o.ID, req.Id)
	if err != nil || m.DeletedAt != nil {
		return nil, store.NotFound("media_file", req.Id)
	}
	now := tx.Now
	m.DeletedAt = &now
	h.logger(ctx).Info("media deleted", "op", "Media.Delete", "media_file_id", m.ID)
	return api.DeleteMedia204Response{}, nil
}
