package models

type contextKey string

const (
	SessionCtxKey   contextKey = "finops_session_context"
	SessionIDCtxKey contextKey = "finops_session_id_context"
	RequestIDCtxKey contextKey = "finops_request_id_context"
	LoggerCtxKey    contextKey = "finops_logger_context"
)

const (
	DefaultAuthCookieName = "finops_session"
)
