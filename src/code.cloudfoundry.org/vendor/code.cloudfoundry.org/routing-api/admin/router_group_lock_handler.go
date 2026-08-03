package admin

import (
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/routing-api/db"
)

type RouterGroupLockHandler struct {
	db     db.DB
	logger lager.Logger
}

func NewRouterGroupLockHandler(database db.DB, logger lager.Logger) *RouterGroupLockHandler {
	return &RouterGroupLockHandler{
		db:     database,
		logger: logger,
	}
}

func (h *RouterGroupLockHandler) LockReads(w http.ResponseWriter, req *http.Request) {
	log := h.logger.Session("router-group-lock-reads")

	log.Info("locking router group reads for backup/restore")
	h.db.LockRouterGroupReads()

	w.WriteHeader(http.StatusOK)
}

func (h *RouterGroupLockHandler) LockWrites(w http.ResponseWriter, req *http.Request) {
	log := h.logger.Session("router-group-lock-writes")

	log.Info("locking router group writes for backup/restore")
	h.db.LockRouterGroupWrites()

	w.WriteHeader(http.StatusOK)
}

func (h *RouterGroupLockHandler) UnlockReads(w http.ResponseWriter, req *http.Request) {
	log := h.logger.Session("router-group-unlock-reads")

	log.Info("unlocking router group reads")
	h.db.UnlockRouterGroupReads()

	w.WriteHeader(http.StatusOK)
}

func (h *RouterGroupLockHandler) UnlockWrites(w http.ResponseWriter, req *http.Request) {
	log := h.logger.Session("router-group-unlock-writes")

	log.Info("unlocking router group writes")
	h.db.UnlockRouterGroupWrites()

	w.WriteHeader(http.StatusOK)
}
