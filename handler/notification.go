package handler

import (
	"log"
	"math"
	"net/http"
	"strconv"

	"igb-leads-go/authctx"
	"igb-leads-go/model"
	"igb-leads-go/repository"
)

// NotificationHandler mirrors NotificationController.
type NotificationHandler struct {
	notify *repository.NotificationRepository
}

func NewNotificationHandler(notify *repository.NotificationRepository) *NotificationHandler {
	return &NotificationHandler{notify: notify}
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, err1 := strconv.Atoi(firstOr(q.Get("page"), "1"))
	limit, err2 := strconv.Atoi(firstOr(q.Get("limit"), "20"))
	if err1 != nil || err2 != nil {
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	unreadOnly := q.Get("unread_only") == "true"
	list, total, err := h.notify.List(r.Context(), authctx.UserID(r), unreadOnly, limit, (page-1)*limit)
	if err != nil {
		log.Printf("Erro ao listar notificações: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	if list == nil {
		list = []model.Notification{}
	}
	var totalPages any
	if limit > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(limit)))
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"data": list,
		"pagination": map[string]any{
			"page": page, "limit": limit, "total": total, "totalPages": totalPages,
		},
	})
}

func (h *NotificationHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	n, err := h.notify.UnreadCount(r.Context(), authctx.UserID(r))
	if err != nil {
		log.Printf("Erro ao contar notificações: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"unread_count": n})
}

func (h *NotificationHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	n, err := h.notify.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Notificação não encontrada")
		return
	}
	if n.UserID != authctx.UserID(r) {
		WriteError(w, http.StatusForbidden, "Acesso negado")
		return
	}
	updated, err := h.notify.MarkRead(r.Context(), n.ID)
	if err != nil {
		log.Printf("Erro ao marcar notificação: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, updated)
}

func (h *NotificationHandler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	if err := h.notify.MarkAllRead(r.Context(), authctx.UserID(r)); err != nil {
		log.Printf("Erro ao marcar todas: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"message": "Todas as notificações marcadas como lidas"})
}

func firstOr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
