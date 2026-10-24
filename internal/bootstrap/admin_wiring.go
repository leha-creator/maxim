package bootstrap

import (
	placementhttp "github.com/leha-creator/maxim/internal/placement/httpapi"
	"github.com/leha-creator/maxim/internal/platform/adminref"
)

// wireAdmin подключает порты ответов операций администратора (этап «API администратора»):
// ссылки на организации и пользователей, выплаты размещения, креатив версии в ОРД.
func (m *modules) wireAdmin() {
	refs := adminref.New(m.in.pool)
	m.placementHTTP.SetAdminPorts(placementhttp.AdminPorts{Refs: refs})
}
