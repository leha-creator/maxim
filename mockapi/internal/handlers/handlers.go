// Package handlers реализует api.StrictServerInterface поверх хранилища в памяти.
package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"

	"maxim/mockapi/internal/api"
	"maxim/mockapi/internal/logx"
	"maxim/mockapi/internal/store"
)

// SessionCookie — имя cookie сессии (07 §4).
const SessionCookie = "session"

// Options — настройки обработчиков.
type Options struct {
	// CookieSecure добавляет признак Secure к cookie сессии; включается на стенде с HTTPS.
	CookieSecure bool
}

// Handlers — реализация всех операций спецификации.
type Handlers struct {
	log  *slog.Logger
	st   *store.Store
	opts Options
}

var _ api.StrictServerInterface = (*Handlers)(nil)

// New создаёт обработчики.
func New(log *slog.Logger, st *store.Store, opts Options) *Handlers {
	return &Handlers{log: log.With("module", "handlers"), st: st, opts: opts}
}

// cookie собирает заголовок Set-Cookie для сессии (07 §4).
func (h *Handlers) cookie(value string, maxAge int) string {
	c := fmt.Sprintf("%s=%s; Path=/; HttpOnly; SameSite=Lax; Max-Age=%d", SessionCookie, value, maxAge)
	if h.opts.CookieSecure {
		c += "; Secure"
	}
	return c
}

// ---------------------------------------------------------------- сессия в context

type sessionKey struct{}

type session struct {
	userID uuid.UUID
	token  string
}

// WithSession кладёт пользователя сессии в context.
func WithSession(ctx context.Context, userID uuid.UUID, token string) context.Context {
	return context.WithValue(ctx, sessionKey{}, session{userID: userID, token: token})
}

// UserFrom возвращает пользователя сессии.
func UserFrom(ctx context.Context) (uuid.UUID, bool) {
	s, ok := ctx.Value(sessionKey{}).(session)
	return s.userID, ok
}

func tokenFrom(ctx context.Context) string {
	s, _ := ctx.Value(sessionKey{}).(session)
	return s.token
}

func (h *Handlers) logger(ctx context.Context) *slog.Logger { return logx.From(ctx, h.log) }

// ---------------------------------------------------------------- доступ

// user возвращает пользователя сессии.
func user(ctx context.Context, tx *store.Tx) (*store.User, error) {
	id, ok := UserFrom(ctx)
	if !ok {
		return nil, store.Unauthorized("session cookie is missing or invalid")
	}
	u, ok := tx.D.Users[id]
	if !ok {
		return nil, store.Unauthorized("session user no longer exists")
	}
	return u, nil
}

// org возвращает организацию пользователя; без реквизитов — 403.
func org(ctx context.Context, tx *store.Tx) (*store.User, *store.Organization, error) {
	u, err := user(ctx, tx)
	if err != nil {
		return nil, nil, err
	}
	if u.OrgID == nil {
		return nil, nil, store.Forbidden("organization details are not filled in")
	}
	o, err := tx.Org(*u.OrgID)
	if err != nil {
		return nil, nil, err
	}
	return u, o, nil
}

// advertiser требует открытый кабинет рекламодателя.
func advertiser(ctx context.Context, tx *store.Tx) (*store.User, *store.Organization, error) {
	u, o, err := org(ctx, tx)
	if err == nil && !o.IsAdvertiser {
		err = store.Forbidden("advertiser cabinet is not open for this organization")
	}
	return u, o, err
}

// blogger требует открытый кабинет блогера.
func blogger(ctx context.Context, tx *store.Tx) (*store.User, *store.Organization, error) {
	u, o, err := org(ctx, tx)
	if err == nil && !o.IsBlogger {
		err = store.Forbidden("blogger cabinet is not open for this organization")
	}
	return u, o, err
}

// ---------------------------------------------------------------- списки

// page отдаёт страницу списка; курсор — смещение. Неверный курсор — с начала.
func page[T any](items []T, limit *int, cursor *string) ([]T, *string) {
	n := 20
	if limit != nil {
		n = *limit
	}
	off := 0
	if cursor != nil {
		if v, err := strconv.Atoi(*cursor); err == nil && v >= 0 {
			off = v
		}
	}
	if off > len(items) {
		off = len(items)
	}
	end := min(off+n, len(items))
	out := items[off:end]
	if out == nil {
		out = []T{}
	}
	if end < len(items) {
		next := strconv.Itoa(end)
		return out, &next
	}
	return out, nil
}

// newestFirst сортирует по убыванию времени создания.
func newestFirst[T any](items []T, at func(T) time.Time) []T {
	out := slices.Clone(items)
	slices.SortStableFunc(out, func(a, b T) int { return at(b).Compare(at(a)) })
	return out
}

// ---------------------------------------------------------------- преобразования

func ptr[T any](v T) *T { return &v }

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func channelRef(c *store.Channel) api.ChannelRef {
	return api.ChannelRef{Id: c.ID, Title: c.Title, IconUrl: strPtr(c.IconURL), Link: strPtr(c.Link)}
}

// mediaURL — адрес картинки-заглушки вместо файла из хранилища.
func mediaURL(m *store.MediaFile, labeled bool) string {
	text := m.Name
	if labeled {
		text = "Реклама · " + m.Name
	}
	return fmt.Sprintf("https://placehold.co/%dx%d/png?text=%s", min(m.Width, 1200), min(m.Height, 1200), url.QueryEscape(text))
}

func mediaFile(m *store.MediaFile) api.MediaFile {
	return api.MediaFile{
		Id: m.ID, OriginalName: m.Name, Mime: api.MediaFileMime(m.Mime), SizeBytes: m.SizeBytes,
		Width: m.Width, Height: m.Height, PreviewUrl: mediaURL(m, false), CreatedAt: m.CreatedAt,
	}
}

// versionMediaURLs — изображения версии в том виде, в каком уйдут в канал.
func versionMediaURLs(tx *store.Tx, orgID uuid.UUID, ids []uuid.UUID, carrier string) []string {
	out := []string{}
	for _, id := range ids {
		for _, m := range tx.D.Media {
			if m.ID == id && m.OrgID == orgID {
				out = append(out, mediaURL(m, carrier == "media"))
			}
		}
	}
	return out
}
