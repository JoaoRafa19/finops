package models

type VerifyResetEmailBody struct {
	Email     string `json:"email"`
	ResetUrl  string `json:"reset_url"`
	VerifyUrl string `json:"verify_url"`
}

const (
	EmailResetKey  = "send_password_reset"
	EmailVerifyKey = "send_email_verify"
)
