package app

import (
	"context"
	"fmt"
	"time"

	"github.com/leha-creator/maxim/internal/marking/domain"
)

// record — объект ОРД любого вида под блокировкой: общий ход отправки и опроса работает
// с полями синхронизации, а сохранение и последствия зависят от вида.
type record interface {
	ref() domain.ObjectRef
	state() *domain.SyncState
	externalID() string
	createdAt() time.Time
	save(ctx context.Context, s *Service, now time.Time) error
}

type orgRecord struct{ v domain.Organization }

func (r *orgRecord) ref() domain.ObjectRef    { return r.v.Ref() }
func (r *orgRecord) state() *domain.SyncState { return &r.v.Sync }
func (r *orgRecord) externalID() string       { return r.v.ExternalID() }
func (r *orgRecord) createdAt() time.Time     { return r.v.CreatedAt }
func (r *orgRecord) save(ctx context.Context, s *Service, now time.Time) error {
	r.v.UpdatedAt = now
	return s.Organizations.Save(ctx, r.v)
}

type contractRecord struct{ v domain.Contract }

func (r *contractRecord) ref() domain.ObjectRef    { return r.v.Ref() }
func (r *contractRecord) state() *domain.SyncState { return &r.v.Sync }
func (r *contractRecord) externalID() string       { return r.v.ExternalID() }
func (r *contractRecord) createdAt() time.Time     { return r.v.CreatedAt }
func (r *contractRecord) save(ctx context.Context, s *Service, now time.Time) error {
	r.v.UpdatedAt = now
	return s.Contracts.Save(ctx, r.v)
}

type platformRecord struct{ v domain.Platform }

func (r *platformRecord) ref() domain.ObjectRef    { return r.v.Ref() }
func (r *platformRecord) state() *domain.SyncState { return &r.v.Sync }
func (r *platformRecord) externalID() string       { return r.v.ExternalID() }
func (r *platformRecord) createdAt() time.Time     { return r.v.CreatedAt }
func (r *platformRecord) save(ctx context.Context, s *Service, now time.Time) error {
	r.v.UpdatedAt = now
	return s.Platforms.Save(ctx, r.v)
}

type creativeRecord struct{ v domain.Creative }

func (r *creativeRecord) ref() domain.ObjectRef    { return r.v.Ref() }
func (r *creativeRecord) state() *domain.SyncState { return &r.v.Sync }
func (r *creativeRecord) externalID() string       { return r.v.ExternalID() }
func (r *creativeRecord) createdAt() time.Time     { return r.v.CreatedAt }
func (r *creativeRecord) save(ctx context.Context, s *Service, now time.Time) error {
	r.v.UpdatedAt = now
	return s.Creatives.Save(ctx, r.v)
}

// lock читает объект с блокировкой строки; нет — domain.ErrNotFound.
func (s *Service) lock(ctx context.Context, ref domain.ObjectRef) (record, error) {
	switch ref.Type {
	case domain.TypeOrganization:
		v, err := s.Organizations.GetForUpdate(ctx, ref.ID)
		return &orgRecord{v}, err
	case domain.TypeContract:
		v, err := s.Contracts.GetForUpdate(ctx, ref.ID)
		return &contractRecord{v}, err
	case domain.TypePlatform:
		v, err := s.Platforms.GetForUpdate(ctx, ref.ID)
		return &platformRecord{v}, err
	case domain.TypeCreative:
		v, err := s.Creatives.GetForUpdate(ctx, ref.ID)
		return &creativeRecord{v}, err
	}
	return nil, fmt.Errorf("marking: unknown object type %q", ref.Type)
}
