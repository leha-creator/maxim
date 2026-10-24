package postgres

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// PayoutDetails — данные организации-блогера для выплаты (модуль payout, 06 §8): тип лица,
// ИНН, ФИО исполнителя и телефон для регистрации в Рокет Ворк.
type PayoutDetails struct {
	ID        uuid.UUID
	LegalType string
	INN       string
	// FullName — ФИО из профиля блогера; пусто — контактное лицо, затем название.
	FullName string
	Phone    string
}

// ForPayout читает данные без блокировки; нет организации — domain.ErrNotFound.
func (r *Organizations) ForPayout(ctx context.Context, id uuid.UUID) (PayoutDetails, error) {
	const op = "Account.Organizations.ForPayout"
	row, err := r.s.q(ctx).GetOrganizationForPayout(ctx, id)
	if err != nil {
		return PayoutDetails{}, r.s.fail(ctx, op, err)
	}
	name := strings.TrimSpace(row.FullName)
	if name == "" {
		name = strings.TrimSpace(row.ContactName)
	}
	if name == "" {
		name = row.Name
	}
	return PayoutDetails{ID: row.ID, LegalType: row.LegalType, INN: row.Inn, FullName: name, Phone: row.Phone}, nil
}
