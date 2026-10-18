package mail

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
)

func TestMultipartMessageAndTemplates(t *testing.T) {
	// Test Subject Encoding
	asciiSubj := encodeSubject("Hello World")
	if asciiSubj != "Hello World" {
		t.Errorf("expected plain ascii subject, got %s", asciiSubj)
	}

	utfSubj := encodeSubject("🚨 Server Alert")
	if !strings.HasPrefix(utfSubj, "=?UTF-8?B?") {
		t.Errorf("expected RFC2047 encoded subject, got %s", utfSubj)
	}

	// Test Templates
	htmlPR, textPR := PasswordResetTemplate("testuser", "https://example.com/reset?token=123")
	if !strings.Contains(htmlPR, "testuser") || !strings.Contains(htmlPR, "https://example.com/reset?token=123") {
		t.Errorf("PasswordResetTemplate missing expected contents in HTML")
	}
	if !strings.Contains(textPR, "15 minutes") {
		t.Errorf("PasswordResetTemplate missing 15 minutes notice in text")
	}

	htmlVerify, textVerify := EmailVerificationTemplate("testuser", "https://example.com/verify?token=abc")
	if !strings.Contains(htmlVerify, "https://example.com/verify?token=abc") {
		t.Errorf("EmailVerificationTemplate missing verify URL in HTML")
	}
	if !strings.Contains(textVerify, "24 hours") {
		t.Errorf("EmailVerificationTemplate missing 24 hours notice in text")
	}

	htmlWelcome, textWelcome := WelcomeEmailTemplate("testuser", "Pro Plan", "https://example.com/login")
	if !strings.Contains(htmlWelcome, "Pro Plan") || !strings.Contains(textWelcome, "Pro Plan") {
		t.Errorf("WelcomeEmailTemplate missing plan name")
	}

	htmlAlert, textAlert := SystemAlertTemplate("CRASH", "Server S1 Stopped", "Fatal exception", map[string]string{"Server": "S1", "Exit Code": "1"}, "https://example.com/servers/S1")
	if !strings.Contains(htmlAlert, "Fatal exception") || !strings.Contains(textAlert, "Exit Code: 1") {
		t.Errorf("SystemAlertTemplate missing alert details")
	}

	// Test MIME builder
	rawMsg := BuildMultipartMessage("Bedrock Manager", "noreply@bsm.local", "player@example.com", "Test Subject", textPR, htmlPR)
	msgStr := string(rawMsg)
	if !strings.Contains(msgStr, "From: Bedrock Manager <noreply@bsm.local>") {
		t.Errorf("expected From header in raw MIME, got:\n%s", msgStr)
	}
	if !strings.Contains(msgStr, "To: player@example.com") {
		t.Errorf("expected To header in raw MIME")
	}
	if !strings.Contains(msgStr, "Content-Type: multipart/alternative;") {
		t.Errorf("expected multipart/alternative Content-Type")
	}
	if !strings.Contains(msgStr, "Content-Type: text/plain; charset=UTF-8") {
		t.Errorf("expected text/plain part")
	}
	if !strings.Contains(msgStr, "Content-Type: text/html; charset=UTF-8") {
		t.Errorf("expected text/html part")
	}
}

// startMockSMTPServer starts an in-memory TCP listener mimicking a basic SMTP server.
func startMockSMTPServer(t *testing.T) (net.Listener, int) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock smtp server: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleMockSMTPConn(conn)
		}
	}()

	addr := listener.Addr().(*net.TCPAddr)
	return listener, addr.Port
}

func handleMockSMTPConn(conn net.Conn) {
	defer conn.Close()
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	_, _ = rw.WriteString("220 mock.bsm.local Service ready\r\n")
	_ = rw.Flush()

	inData := false

	for {
		line, err := rw.ReadString('\n')
		if err != nil {
			return
		}
		trimmed := strings.TrimRight(line, "\r\n")

		if inData {
			if trimmed == "." {
				inData = false
				_, _ = rw.WriteString("250 2.0.0 OK: message queued\r\n")
				_ = rw.Flush()
			}
			continue
		}

		upper := strings.ToUpper(trimmed)
		switch {
		case strings.HasPrefix(upper, "EHLO") || strings.HasPrefix(upper, "HELO"):
			_, _ = rw.WriteString("250-mock.bsm.local\r\n250-AUTH PLAIN LOGIN\r\n250 8BITMIME\r\n")
			_ = rw.Flush()
		case strings.HasPrefix(upper, "AUTH PLAIN"):
			_, _ = rw.WriteString("235 2.7.0 Authentication successful\r\n")
			_ = rw.Flush()
		case strings.HasPrefix(upper, "MAIL FROM:"):
			_, _ = rw.WriteString("250 2.1.0 Sender OK\r\n")
			_ = rw.Flush()
		case strings.HasPrefix(upper, "RCPT TO:"):
			_, _ = rw.WriteString("250 2.1.5 Recipient OK\r\n")
			_ = rw.Flush()
		case strings.HasPrefix(upper, "DATA"):
			inData = true
			_, _ = rw.WriteString("354 Start mail input; end with <CRLF>.<CRLF>\r\n")
			_ = rw.Flush()
		case strings.HasPrefix(upper, "QUIT"):
			_, _ = rw.WriteString("221 2.0.0 Bye\r\n")
			_ = rw.Flush()
			return
		case strings.HasPrefix(upper, "RSET"):
			_, _ = rw.WriteString("250 2.0.0 OK\r\n")
			_ = rw.Flush()
		default:
			_, _ = rw.WriteString("500 5.5.1 Command unrecognized\r\n")
			_ = rw.Flush()
		}
	}
}

func TestSendMailMock(t *testing.T) {
	listener, port := startMockSMTPServer(t)
	defer listener.Close()

	cfg := &Config{
		Enabled:       true,
		Host:          "127.0.0.1",
		Port:          port,
		Encryption:    "none",
		AuthType:      "plain",
		Username:      "bsm_admin",
		Password:      "bsm_pass",
		FromEmail:     "noreply@bsm.local",
		FromName:      "BSM Alerting",
		SkipTLSVerify: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rawMsg := BuildMultipartMessage(cfg.FromName, cfg.FromEmail, "test@example.com", "Mock Alert", "Plain content", "<p>HTML content</p>")
	err := SendMail(ctx, cfg, []string{"test@example.com"}, rawMsg)
	if err != nil {
		t.Fatalf("SendMail failed against mock server: %v", err)
	}
}

func TestMailerWithDatabase(t *testing.T) {
	db, err := database.OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Seed settings
	_ = db.SetSetting(ctx, SettingSMTPEnabled, "true")
	_ = db.SetSetting(ctx, SettingSMTPHost, "127.0.0.1")
	_ = db.SetSetting(ctx, SettingSMTPPort, "2525")
	_ = db.SetSetting(ctx, SettingSMTPEncryption, "none")
	_ = db.SetSetting(ctx, SettingSMTPFromEmail, "noreply@bsm.local")
	_ = db.SetSetting(ctx, SettingAppURL, "https://bsm.local")

	cfg, err := LoadConfig(ctx, db)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if !cfg.Enabled || cfg.Host != "127.0.0.1" || cfg.Port != 2525 || cfg.AppURL != "https://bsm.local" {
		t.Errorf("unexpected loaded config: %+v", cfg)
	}

	mailer := NewMailer(db)
	defer mailer.Close()

	// Enqueue jobs (should not block or panic)
	mailer.SendPasswordReset("user@example.com", "user", "https://bsm.local/reset-password?token=tok")
	mailer.SendEmailVerification("user@example.com", "user", "https://bsm.local/verify-email?token=tok")
	mailer.SendWelcomeEmail("user@example.com", "user", "Default Plan", "https://bsm.local")

	// Allow brief time for worker loop to pick up without crash
	time.Sleep(50 * time.Millisecond)
}
