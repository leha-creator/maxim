package handlers

import (
	"context"
	"net/http"

	"maxim/mockapi/internal/api"
	"maxim/mockapi/internal/store"
)

// Операции администратора (тег admin-channels) тестовый сервер не реализует:
// модерацию в нём заменяют начальные данные. Ответ — 501, как у бэкенда до реализации.

func notImplemented(ctx context.Context, h *Handlers, op string) error {
	h.logger(ctx).Debug("operation is not implemented in mock", "op", "Admin."+op)
	return &store.Error{Status: http.StatusNotImplemented, Code: "not_implemented",
		Message: "operation is not implemented in mock server", Details: map[string]any{"operation": op}}
}

func (h *Handlers) ListAdminChannels(ctx context.Context, _ api.ListAdminChannelsRequestObject) (api.ListAdminChannelsResponseObject, error) {
	return nil, notImplemented(ctx, h, "listAdminChannels")
}

func (h *Handlers) GetAdminChannel(ctx context.Context, _ api.GetAdminChannelRequestObject) (api.GetAdminChannelResponseObject, error) {
	return nil, notImplemented(ctx, h, "getAdminChannel")
}

func (h *Handlers) ApproveChannel(ctx context.Context, _ api.ApproveChannelRequestObject) (api.ApproveChannelResponseObject, error) {
	return nil, notImplemented(ctx, h, "approveChannel")
}

func (h *Handlers) RejectChannel(ctx context.Context, _ api.RejectChannelRequestObject) (api.RejectChannelResponseObject, error) {
	return nil, notImplemented(ctx, h, "rejectChannel")
}

func (h *Handlers) SetChannelRknCheck(ctx context.Context, _ api.SetChannelRknCheckRequestObject) (api.SetChannelRknCheckResponseObject, error) {
	return nil, notImplemented(ctx, h, "setChannelRknCheck")
}
