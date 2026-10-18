package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
	"unicode"
)

// LoginAuth implements the LOGIN SASL authentication mechanism.
type LoginAuth struct {
	username, password string
}

func NewLoginAuth(username, password string) smtp.Auth {
	return &LoginAuth{username: username, password: password}
}

func (a *LoginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}

func (a *LoginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		prompt := strings.ToLower(strings.TrimSpace(string(fromServer)))
		if strings.Contains(prompt, "user") || prompt == "vxnlcm5hbwu6" {
			return []byte(a.username), nil
		} else if strings.Contains(prompt, "pass") || prompt == "cGFzc3dvcmQ6" {
			return []byte(a.password), nil
		}

		if decoded, err := base64.StdEncoding.DecodeString(string(fromServer)); err == nil {
			dPrompt := strings.ToLower(strings.TrimSpace(string(decoded)))
			if strings.Contains(dPrompt, "username") || strings.Contains(dPrompt, "user") {
				return []byte(a.username), nil
			} else if strings.Contains(dPrompt, "password") || strings.Contains(dPrompt, "pass") {
				return []byte(a.password), nil
			}
		}

		// Fallback to username first
		return []byte(a.username), nil
	}
	return nil, nil
}

// encodeSubject returns RFC 2047 base64 encoded subject if non-ASCII characters exist.
func encodeSubject(s string) string {
	isASCII := true
	for _, r := range s {
		if r > unicode.MaxASCII {
			isASCII = false
			break
		}
	}
	if isASCII {
		return s
	}
	return fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(s)))
}

// BuildMultipartMessage builds an RFC 2046 multipart/alternative MIME message.
func BuildMultipartMessage(fromName, fromEmail, toEmail, subject, textBody, htmlBody string) []byte {
	boundary := fmt.Sprintf("bsm_part_%d", time.Now().UnixNano())
	var buf bytes.Buffer

	fromHeader := fmt.Sprintf("%s <%s>", fromName, fromEmail)
	if fromName == "" {
		fromHeader = fromEmail
	}

	buf.WriteString(fmt.Sprintf("From: %s\r\n", fromHeader))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", toEmail))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", encodeSubject(subject)))
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z)))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary))

	// Plain text alternative
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	buf.WriteString(textBody)
	buf.WriteString("\r\n\r\n")

	// HTML alternative
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	buf.WriteString(htmlBody)
	buf.WriteString("\r\n\r\n")

	buf.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	return buf.Bytes()
}

// SendMail connects to the configured SMTP server and sends the message.
func SendMail(ctx context.Context, cfg *Config, recipients []string, message []byte) error {
	if cfg.Host == "" {
		return fmt.Errorf("smtp host is not configured")
	}
	if len(recipients) == 0 {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	var client *smtp.Client
	var err error

	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.SkipTLSVerify,
		ServerName:         cfg.Host,
	}

	switch cfg.Encryption {
	case "ssl_tls":
		conn, dialErr := tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
		if dialErr != nil {
			return fmt.Errorf("tls dial to %s failed: %w", addr, dialErr)
		}
		defer conn.Close()
		client, err = smtp.NewClient(conn, cfg.Host)
		if err != nil {
			return fmt.Errorf("smtp handshake failed: %w", err)
		}

	case "none":
		conn, dialErr := dialer.DialContext(ctx, "tcp", addr)
		if dialErr != nil {
			return fmt.Errorf("dial to %s failed: %w", addr, dialErr)
		}
		defer conn.Close()
		client, err = smtp.NewClient(conn, cfg.Host)
		if err != nil {
			return fmt.Errorf("smtp handshake failed: %w", err)
		}

	default: // "starttls"
		conn, dialErr := dialer.DialContext(ctx, "tcp", addr)
		if dialErr != nil {
			return fmt.Errorf("dial to %s failed: %w", addr, dialErr)
		}
		defer conn.Close()
		client, err = smtp.NewClient(conn, cfg.Host)
		if err != nil {
			return fmt.Errorf("smtp handshake failed: %w", err)
		}

		if ok, _ := client.Extension("STARTTLS"); ok {
			if err = client.StartTLS(tlsConfig); err != nil {
				return fmt.Errorf("starttls negotiation failed: %w", err)
			}
		}
	}
	defer client.Quit()

	// Handle Authentication if not anonymous and credentials provided
	if cfg.AuthType != "anonymous" && cfg.Username != "" {
		var auth smtp.Auth
		if cfg.AuthType == "login" {
			auth = NewLoginAuth(cfg.Username, cfg.Password)
		} else {
			auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		}

		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp authentication failed: %w", err)
		}
	}

	sender := cfg.FromEmail
	if sender == "" {
		sender = cfg.Username
	}
	if sender == "" {
		sender = "bsm@localhost"
	}

	if err := client.Mail(sender); err != nil {
		return fmt.Errorf("smtp MAIL FROM failed: %w", err)
	}

	for _, rcpt := range recipients {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp RCPT TO <%s> failed: %w", rcpt, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA command failed: %w", err)
	}

	if _, err := w.Write(message); err != nil {
		_ = w.Close()
		return fmt.Errorf("writing email body failed: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("closing email data stream failed: %w", err)
	}

	return nil
}
