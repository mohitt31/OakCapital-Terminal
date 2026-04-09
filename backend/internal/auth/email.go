package auth

import (
	"fmt"
	"net/smtp"
)

type EmailService struct {
	host string
	port string
	user string
	pass string
}

func NewEmailService(host, port, user, pass string) *EmailService {
	return &EmailService{
		host: host,
		port: port,
		user: user,
		pass: pass,
	}
}

func (s *EmailService) SendPasswordResetEmail(to, resetToken string) error {
	if s.host == "" || s.user == "" {
		return fmt.Errorf("SMTP not configured")
	}

	from := s.user
	subject := "Password Reset Request"
	body := fmt.Sprintf(`Hi,

You requested a password reset for your Opensoft account.

Your reset token is: %s

This token expires in 15 minutes. Use it to reset your password via the /auth/reset-password endpoint.

If you didn't request this, please ignore this email.

Best,
Opensoft Team`, resetToken)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, to, subject, body)

	auth := smtp.PlainAuth("", s.user, s.pass, s.host)
	addr := s.host + ":" + s.port

	err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (s *EmailService) SendDeleteAccountEmail(to, deleteToken string) error {
	if s.host == "" || s.user == "" {
		return fmt.Errorf("SMTP not configured")
	}

	from := s.user
	subject := "Confirm Account Deletion"
	body := fmt.Sprintf(`Hi,

You (or someone else) requested to delete your Opensoft account.

Your deletion confirmation token is: %s

This token expires in 15 minutes. Use it to confirm your account deletion via the /auth/account endpoint.

If you didn't request this, PLEASE IGNORE THIS EMAIL.

Best,
Opensoft Team`, deleteToken)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, to, subject, body)

	auth := smtp.PlainAuth("", s.user, s.pass, s.host)
	addr := s.host + ":" + s.port

	err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
