package httpx_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/leha-creator/maxim/internal/platform/apperr"
	"github.com/leha-creator/maxim/internal/platform/httpx"
	"github.com/leha-creator/maxim/internal/platform/logx"
)

func TestRequireAdmin(t *testing.T) {
	log := logx.Discard()
	if _, err := httpx.RequireAdmin(context.Background(), log, "Test", "moderator"); !apperr.Is(err, apperr.CodeUnauthorized) {
		t.Fatalf("no session: %v", err)
	}
	for role, ok := range map[string]bool{"": false, "moderator": true, "senior": true, "owner": false} {
		ctx := httpx.WithPrincipal(context.Background(), &httpx.Principal{UserID: uuid.New(), AdminRole: role})
		_, err := httpx.RequireAdmin(ctx, log, "Test", "moderator", "senior")
		if ok && err != nil {
			t.Errorf("%q: %v", role, err)
		}
		if !ok {
			ae, isApp := apperr.As(err)
			if !isApp || ae.Code != apperr.CodeForbidden || ae.Details["reason"] != httpx.ReasonAdminRequired {
				t.Errorf("%q: %v", role, err)
			}
		}
	}
}
