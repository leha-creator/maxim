package handlers

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"maxim/mockapi/internal/api"
	"maxim/mockapi/internal/store"
)

// Общие преобразования для операций администратора.

// recast переносит общие поля между сгенерированными типами (allOf разворачивается в плоские
// структуры с одинаковыми JSON-именами).
func recast[T any](v any) T {
	var out T
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	return out
}

// isSenior — пользователь видит полные ИНН, почты и телефоны (R-14).
func isSenior(u *store.User) bool { return u.AdminRole == "senior" }

func innFor(u *store.User, inn string) string {
	if isSenior(u) {
		return inn
	}
	return maskINN(inn)
}

// maskEmail оставляет первую букву и домен: a***@example.test.
func maskEmail(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return "***"
	}
	return email[:1] + "***" + email[at:]
}

func emailFor(u *store.User, email string) string {
	if isSenior(u) {
		return email
	}
	return maskEmail(email)
}

// maskPhone оставляет код страны и две последние цифры.
func maskPhone(phone string) string {
	if len(phone) < 5 {
		return "***"
	}
	return phone[:2] + strings.Repeat("*", len(phone)-4) + phone[len(phone)-2:]
}

func orgRef(tx *store.Tx, id uuid.UUID) api.AdminOrganizationRef {
	o, err := tx.Org(id)
	if err != nil {
		return api.AdminOrganizationRef{Id: id}
	}
	return api.AdminOrganizationRef{Id: o.ID, Name: o.Name, Inn: maskINN(o.INN)}
}

func userRef(tx *store.Tx, id uuid.UUID) api.AdminUserRef {
	if u, ok := tx.D.Users[id]; ok {
		return api.AdminUserRef{Id: id, Email: u.Email}
	}
	return api.AdminUserRef{Id: id, Email: "unknown"}
}

func userRefPtr(tx *store.Tx, id *uuid.UUID) *api.AdminUserRef {
	if id == nil {
		return nil
	}
	r := userRef(tx, *id)
	return &r
}

// moscowDay — начало суток по Москве для фильтров from/to (to — включительно).
func moscowDay(d *openapi_types.Date, next bool) *time.Time {
	if d == nil {
		return nil
	}
	t := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, store.MSK)
	if next {
		t = t.AddDate(0, 0, 1)
	}
	return &t
}

// inPeriod — момент попадает в [from, to] по Москве.
func inPeriod(at time.Time, from, to *openapi_types.Date) bool {
	if f := moscowDay(from, false); f != nil && at.Before(*f) {
		return false
	}
	if t := moscowDay(to, true); t != nil && !at.Before(*t) {
		return false
	}
	return true
}

func dateOf(t time.Time) openapi_types.Date {
	return openapi_types.Date{Time: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)}
}

func reasonBody(r *api.AdminReasonInput) string {
	if r == nil {
		return ""
	}
	return r.Reason
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
