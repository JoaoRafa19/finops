package models

type contextKey string

const (
	SessionCtxKey   contextKey = "finops_session_context"
	SessionIDCtxKey contextKey = "finops_session_id_context"
)

const (
	DefaultAuthCookieName = "finops_session"
)
