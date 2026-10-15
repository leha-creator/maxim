package handlers

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"slices"
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

// applyCampaignInput переносит поля в кампанию и проверяет период (A-39).
func applyCampaignInput(tx *store.Tx, c *store.Campaign, in *api.CampaignInput) error {
	periodChanged := in.DateFrom != nil || in.DateTo != nil || in.TimeFrom != nil || in.TimeTo != nil
	if periodChanged && c.Status != "draft" {
		return store.Conflict("conflict", "campaign period can be changed only in draft", map[string]any{"status": c.Status})
	}
	if in.Name != nil {
		c.Name = *in.Name
	}
	if in.ExtraRequirements != nil {
		c.ExtraRequirements = in.ExtraRequirements
	}
	if in.DateFrom != nil {
		c.DateFrom = &in.DateFrom.Time
	}
	if in.DateTo != nil {
		c.DateTo = &in.DateTo.Time
	}
	if in.TimeFrom != nil {
		c.TimeFrom = in.TimeFrom
	}
	if in.TimeTo != nil {
		c.TimeTo = in.TimeTo
	}
	if c.DateFrom != nil && c.DateTo != nil && c.DateTo.Before(*c.DateFrom) {
		return store.Validation("date_to", "date_to is before date_from")
	}
	if c.TimeFrom != nil && c.TimeTo != nil && *c.TimeTo <= *c.TimeFrom {
		return store.Validation("time_to", "time_to must be after time_from")
	}
	if in.DateFrom != nil {
		today := tx.Now.In(store.MSK)
		if in.DateFrom.Time.Before(time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)) {
			return store.Validation("date_from", "date_from is in the past")
		}
	}
	c.UpdatedAt = tx.Now
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
	pg, next := page(mine, req.Params.Limit, req.Params.Cursor)
	items := []api.Campaign{}
	for _, c := range pg {
		items = append(items, campaign(c))
	}
	return api.ListCampaigns200JSONResponse{Items: items, NextCursor: next}, nil
}

func (h *Handlers) CreateCampaign(ctx context.Context, req api.CreateCampaignRequestObject) (api.CreateCampaignResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	_, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	if req.Body.Name == nil || *req.Body.Name == "" {
		return nil, store.Validation("name", "name is required")
	}
	c := &store.Campaign{ID: store.NewID(), OrgID: o.ID, Status: "draft", CreatedAt: tx.Now,
		Post: store.PostDraft{Doc: api.EditorDoc{Type: "doc", Content: []api.EditorBlock{}}, MediaIDs: []api.Uuid{},
			TokenCarrier: "text", TokenPosition: "below", UpdatedAt: tx.Now}}
	if err := applyCampaignInput(tx, c, req.Body); err != nil {
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
	if err := applyCampaignInput(tx, c, req.Body); err != nil {
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
	pg, next := page(mine, req.Params.Limit, req.Params.Cursor)
	items := []api.MediaFile{}
	for _, m := range pg {
		items = append(items, mediaFile(m))
	}
	return api.ListMedia200JSONResponse{Items: items, NextCursor: next}, nil
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
