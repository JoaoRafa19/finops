package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"

	emailtemplates "finops/internal/web/templates/email"
)

type EmailService interface {
	SendPasswordReset(ctx context.Context, toEmail, resetURL string) error
	SendVerifyEmail(ctx context.Context, toEmail, verifyURL string) error
}

// SMTPEmailService envia emails via SMTP com STARTTLS ou TLS.
type SMTPEmailService struct {
	host     string
	port     string
	user     string
	password string
	from     string
}

func NewSMTPEmailService(host, port, user, password, from string) EmailService {
	return &SMTPEmailService{host: host, port: port, user: user, password: password, from: from}
}

func (s *SMTPEmailService) SendPasswordReset(ctx context.Context, toEmail, resetURL string) error {
	plain := fmt.Sprintf(
		"Olá,\r\n\r\nRecebemos uma solicitação para redefinir a senha da sua conta Finops.\r\n\r\nAcesse o link abaixo para criar uma nova senha (válido por 1 hora):\r\n\r\n%s\r\n\r\nSe você não solicitou isso, ignore este email.\r\n\r\nEquipe Finops",
		resetURL,
	)
	var htmlBuf bytes.Buffer
	if err := emailtemplates.PasswordReset(resetURL).Render(ctx, &htmlBuf); err != nil {
		return fmt.Errorf("render email template: %w", err)
	}
	return s.sendMultipart(toEmail, "Redefinição de senha — Finops", plain, htmlBuf.String())
}

func (s *SMTPEmailService) SendVerifyEmail(ctx context.Context, toEmail, verifyURL string) error {
	plain := fmt.Sprintf(
		"Olá,\r\n\r\nBem-vindo ao Finops! Confirme seu e-mail acessando o link abaixo (válido por 24 horas):\r\n\r\n%s\r\n\r\nSe você não criou esta conta, ignore este e-mail.\r\n\r\nEquipe Finops",
		verifyURL,
	)
	var htmlBuf bytes.Buffer
	if err := emailtemplates.VerifyEmail(verifyURL).Render(ctx, &htmlBuf); err != nil {
		return fmt.Errorf("render email template: %w", err)
	}
	return s.sendMultipart(toEmail, "Confirme seu e-mail — Finops", plain, htmlBuf.String())
}

// sendMultipart envia um multipart/alternative (text/plain + text/html) com
// bodies em base64 chunked para escapar dos reescreves de linha dos relays.
func (s *SMTPEmailService) sendMultipart(toEmail, subject, plain, html string) error {
	encSubject := mime.QEncoding.Encode("UTF-8", subject)
	boundary := fmt.Sprintf("=_%d_finops", time.Now().UnixNano())
	msgID := fmt.Sprintf("<%d.finops@%s>", time.Now().UnixNano(), s.host)
	fromHeader := mime.QEncoding.Encode("UTF-8", "Finops") + " <" + s.from + ">"

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMessage-ID: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n"+
			"--%s\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n\r\n"+
			"--%s\r\nContent-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n\r\n"+
			"--%s--\r\n",
		fromHeader, toEmail, encSubject, msgID, time.Now().Format(time.RFC1123Z), boundary,
		boundary, base64Chunked(plain),
		boundary, base64Chunked(html),
		boundary,
	)

	addr := net.JoinHostPort(s.host, s.port)
	auth := smtp.PlainAuth("", s.user, s.password, s.host)
	if s.port == "465" {
		return s.sendTLS(addr, auth, toEmail, msg)
	}
	return smtp.SendMail(addr, auth, s.from, []string{toEmail}, []byte(msg))
}

// base64Chunked encodes the payload as base64 and breaks it into 76-char lines
// (RFC 2045) so relays don't inject CRLF in the middle of long lines.
func base64Chunked(s string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(s))
	var b strings.Builder
	for i := 0; i < len(encoded); i += 76 {
		end := min(i+76, len(encoded))
		b.WriteString(encoded[i:end])
		b.WriteString("\r\n")
	}
	return b.String()
}

func (s *SMTPEmailService) sendTLS(addr string, auth smtp.Auth, to, msg string) error {
	tlsCfg := &tls.Config{ServerName: s.host}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return err
	}
	defer client.Close()
	if err = client.Auth(auth); err != nil {
		return err
	}
	if err = client.Mail(s.from); err != nil {
		return err
	}
	if err = client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = strings.NewReader(msg).WriteTo(w); err != nil {
		return err
	}
	return w.Close()
}

// NoopEmailService loga o link em vez de enviar email — usado em desenvolvimento.
type NoopEmailService struct{}

func NewNoopEmailService() EmailService { return &NoopEmailService{} }

func (n *NoopEmailService) SendPasswordReset(_ context.Context, toEmail, resetURL string) error {
	slog.Info("password_reset_link", "to", toEmail, "url", resetURL)
	return nil
}

func (n *NoopEmailService) SendVerifyEmail(_ context.Context, toEmail, verifyURL string) error {
	slog.Info("verify_email_link", "to", toEmail, "url", verifyURL)
	return nil
}
