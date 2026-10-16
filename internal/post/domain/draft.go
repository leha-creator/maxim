package domain

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

// PostStatus — состояние поста (04 §4).
type PostStatus string

const (
	PostDraft    PostStatus = "draft"
	PostArchived PostStatus = "archived"
)

// DefaultAlbumLimit — рабочий предел изображений в посте (A-23, Q-05).
const DefaultAlbumLimit = 10

// Причины отказа в сохранении черновика (R-11, R-15, details.reason).
var (
	ErrDraftTooLong     = &RuleError{Field: "content_doc", Reason: "too_long"}
	ErrDuplicateMedia   = &RuleError{Field: "media_file_ids", Reason: "duplicate_media"}
	ErrTooManyMedia     = &RuleError{Field: "media_file_ids", Reason: "too_many_media"}
	ErrMediaNotFound    = &RuleError{Field: "media_file_ids", Reason: "media_not_found"}
	ErrInvalidTokenMode = &RuleError{Field: "token_carrier", Reason: "invalid"}
)

// Draft — редактируемое содержимое поста: документ, изображения по порядку, место токена.
type Draft struct {
	Doc           Doc
	MediaIDs      []uuid.UUID
	TokenCarrier  TokenCarrier
	TokenPosition TokenPosition
}

// Post — единый пост кампании (04 §4, D-16).
type Post struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	CampaignID     uuid.UUID
	Status         PostStatus
	Draft          Draft
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewPost — пустой пост новой кампании: без текста и изображений, пометка в тексте, erid под постом.
func NewPost(id, orgID, campaignID uuid.UUID, now time.Time) Post {
	return Post{
		ID: id, OrganizationID: orgID, CampaignID: campaignID, Status: PostDraft,
		Draft:     Draft{TokenCarrier: CarrierText, TokenPosition: PositionBelow},
		CreatedAt: now, UpdatedAt: now,
	}
}

// DraftInput — сохраняемый черновик. Doc уже нормализован (Normalize, Autolink).
// Не переданное место токена получает значение по умолчанию (R-13, D-30).
type DraftInput struct {
	Doc           Doc
	MediaIDs      []uuid.UUID
	TokenCarrier  *TokenCarrier
	TokenPosition *TokenPosition
}

// SaveDraft заменяет черновик (R-11, R-13, R-15). Принадлежность файлов
// организации проверяет app; здесь — повторы, предел альбома и предел сохранения.
// Недоступное место токена не отклоняется: его показывает предпросмотр.
func (p *Post) SaveDraft(in DraftInput, albumLimit int, now time.Time) error {
	if len(in.MediaIDs) > albumLimit {
		return ErrTooManyMedia
	}
	seen := make(map[uuid.UUID]bool, len(in.MediaIDs))
	for _, id := range in.MediaIDs {
		if seen[id] {
			return ErrDuplicateMedia
		}
		seen[id] = true
	}
	if Length(Serialize(in.Doc, nil)) > SaveLimit {
		return ErrDraftTooLong
	}
	d := Draft{Doc: in.Doc, MediaIDs: slices.Clone(in.MediaIDs), TokenCarrier: CarrierText, TokenPosition: PositionBelow}
	if len(in.MediaIDs) > 0 {
		d.TokenCarrier = CarrierMedia
	}
	if in.TokenCarrier != nil {
		if !in.TokenCarrier.Valid() {
			return ErrInvalidTokenMode
		}
		d.TokenCarrier = *in.TokenCarrier
	}
	if in.TokenPosition != nil {
		if !in.TokenPosition.Valid() {
			return &RuleError{Field: "token_position", Reason: "invalid"}
		}
		d.TokenPosition = *in.TokenPosition
	}
	p.Draft = d
	p.UpdatedAt = now
	return nil
}

// RemoveMedia убирает файл из черновика; false — файла в черновике не было.
func (p *Post) RemoveMedia(id uuid.UUID) bool {
	i := slices.Index(p.Draft.MediaIDs, id)
	if i < 0 {
		return false
	}
	p.Draft.MediaIDs = slices.Delete(slices.Clone(p.Draft.MediaIDs), i, i+1)
	return true
}
