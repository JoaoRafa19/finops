package models

import "time"

type Session struct {
	ID         string    `json:"id"`
	UserID     int64     `json:"user_id"`
	Email      string    `json:"email"`
	CSRFToken  string    `json:"csrf_token"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	RememberMe bool      `json:"remember_me"`
}
