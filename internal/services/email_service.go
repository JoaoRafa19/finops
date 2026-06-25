package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"time"

	emailtemplates "finops/internal/web/templates/email"
)

type EmailService interface {
	SendPasswordReset(ctx context.Context, toEmail, resetURL string) error
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
	subject := "Redefinição de senha — Finops"
	plain := fmt.Sprintf(
		"Olá,\r\n\r\nRecebemos uma solicitação para redefinir a senha da sua conta Finops.\r\n\r\nAcesse o link abaixo para criar uma nova senha (válido por 1 hora):\r\n\r\n%s\r\n\r\nSe você não solicitou isso, ignore este email.\r\n\r\nEquipe Finops",
		resetURL,
	)

	var htmlBuf bytes.Buffer
	if err := emailtemplates.PasswordReset(resetURL).Render(ctx, &htmlBuf); err != nil {
		return fmt.Errorf("render email template: %w", err)
	}

	boundary := fmt.Sprintf("=_%d_finops", time.Now().UnixNano())
	msgID := fmt.Sprintf("<%d.finops@%s>", time.Now().UnixNano(), s.host)

	msg := fmt.Sprintf(
		"From: Finops <%s>\r\nTo: %s\r\nSubject: %s\r\nMessage-ID: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n"+
			"--%s\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n%s\r\n\r\n"+
			"--%s\r\nContent-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n%s\r\n\r\n"+
			"--%s--\r\n",
		s.from, toEmail, subject, msgID, time.Now().Format(time.RFC1123Z), boundary,
		boundary, plain,
		boundary, htmlBuf.String(),
		boundary,
	)

	addr := net.JoinHostPort(s.host, s.port)
	auth := smtp.PlainAuth("", s.user, s.password, s.host)

	if s.port == "465" {
		return s.sendTLS(addr, auth, toEmail, msg)
	}
	return smtp.SendMail(addr, auth, s.from, []string{toEmail}, []byte(msg))
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
