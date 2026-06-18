package web

import (
	"finops/internal/observability"
	service "finops/internal/services"
	"finops/internal/web/middleware"
	"finops/internal/web/render"
	"finops/internal/web/templates"
	"net/http"
	"strings"
)

type ChatController struct {
	chatSvc service.ChatService
}

func NewChatController(chatSvc service.ChatService) *ChatController {
	return &ChatController{chatSvc: chatSvc}
}

func (c *ChatController) Message(w http.ResponseWriter, r *http.Request) {
	logger := observability.Logger(r.Context())
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "requisição inválida", http.StatusBadRequest)
		return
	}

	question := strings.TrimSpace(r.FormValue("message"))
	if question == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	answer, err := c.chatSvc.Ask(r.Context(), session.UserID, question)
	if err != nil {
		logger.Error("chat_ask_failed", "user_id", session.UserID, "error", err)
		render.Templ(w, r, http.StatusOK, templates.ChatErrorBubble("Não foi possível processar sua pergunta. Tente novamente."))
		return
	}

	render.Templ(w, r, http.StatusOK, templates.ChatMessagePair(question, answer))
}

func (c *ChatController) GetHistory(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	history, err := c.chatSvc.History(r.Context(), session.UserID)
	if err != nil {
		render.Templ(w, r, http.StatusOK, templates.ChatHistoryFragment(nil))
		return
	}

	render.Templ(w, r, http.StatusOK, templates.ChatHistoryFragment(history))
}
