package mailer

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type SMTP struct {
	Host                 string
	Port                 int
	User, Password, From string
}

func FromEnv(host, port, user, password, from string) (*SMTP, error) {
	if strings.TrimSpace(user) == "" || strings.TrimSpace(password) == "" {
		return nil, nil
	}
	parsed, err := strconv.Atoi(port)
	if err != nil || (parsed != 465 && parsed != 587) {
		return nil, errors.New("SMTP_PORT must be 465 or 587")
	}
	if strings.TrimSpace(host) == "" {
		host = "smtp.hostinger.com"
	}
	if strings.TrimSpace(from) == "" {
		from = user
	}
	fromAddress, err := mail.ParseAddress(strings.TrimSpace(from))
	if err != nil || fromAddress.Address != strings.TrimSpace(from) {
		return nil, errors.New("SMTP_FROM_EMAIL must be one plain email address")
	}
	return &SMTP{Host: strings.TrimSpace(host), Port: parsed, User: strings.TrimSpace(user), Password: password, From: strings.TrimSpace(from)}, nil
}

func (m *SMTP) SendHTML(ctx context.Context, to, subject, htmlBody, messageID string) error {
	if m == nil {
		return errors.New("SMTP is not configured")
	}
	toAddress, err := mail.ParseAddress(strings.TrimSpace(to))
	if err != nil || toAddress.Address != strings.TrimSpace(to) {
		return errors.New("recipient must be one plain email address")
	}
	to = toAddress.Address
	if strings.ContainsAny(subject, "\r\n") || strings.ContainsAny(messageID, "\r\n<>") || strings.TrimSpace(messageID) == "" {
		return errors.New("invalid email header")
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	address := net.JoinHostPort(m.Host, strconv.Itoa(m.Port))
	tlsConfig := &tls.Config{ServerName: m.Host, MinVersion: tls.VersionTLS12}
	var client *smtp.Client
	if m.Port == 465 {
		conn, err := tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
		if err != nil {
			return fmt.Errorf("dial smtp tls: %w", err)
		}
		_ = conn.SetDeadline(deadline(ctx))
		client, err = smtp.NewClient(conn, m.Host)
		if err != nil {
			_ = conn.Close()
			return err
		}
	} else {
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return fmt.Errorf("dial smtp: %w", err)
		}
		_ = conn.SetDeadline(deadline(ctx))
		client, err = smtp.NewClient(conn, m.Host)
		if err != nil {
			_ = conn.Close()
			return err
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			_ = client.Close()
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}
	defer client.Close()
	if err := client.Auth(smtp.PlainAuth("", m.User, m.Password, m.Host)); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := client.Mail(m.From); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	headers := fmt.Sprintf("From: Tawafiq Hub <%s>\r\nTo: %s\r\nSubject: %s\r\nMessage-ID: <%s>\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n", m.From, to, subject, messageID)
	if _, err := io.Copy(w, bufio.NewReader(strings.NewReader(headers+htmlBody))); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func deadline(ctx context.Context) time.Time {
	if value, ok := ctx.Deadline(); ok {
		return value
	}
	return time.Now().Add(15 * time.Second)
}
