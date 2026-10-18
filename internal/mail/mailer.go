package mail

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
)

// EmailJob encapsulates an asynchronous outbound email dispatch.
type EmailJob struct {
	Recipients []string
	Subject    string
	HTMLBody   string
	TextBody   string
}

// Mailer manages asynchronous queueing and synchronous dispatch of system emails.
type Mailer struct {
	db       *database.ManagerDB
	jobs     chan EmailJob
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewMailer instantiates a Mailer with an active background worker pool.
func NewMailer(db *database.ManagerDB) *Mailer {
	m := &Mailer{
		db:       db,
		jobs:     make(chan EmailJob, 100),
		stopChan: make(chan struct{}),
	}

	m.wg.Add(1)
	go m.worker()

	return m
}

func (m *Mailer) worker() {
	defer m.wg.Done()
	for {
		select {
		case <-m.stopChan:
			return
		case job, ok := <-m.jobs:
			if !ok {
				return
			}
			m.processJob(job)
		}
	}
}

func (m *Mailer) processJob(job EmailJob) {
	if len(job.Recipients) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cfg, err := LoadConfig(ctx, m.db)
	if err != nil {
		log.Printf("[MAILER ERROR] Failed to load SMTP config for job: %v", err)
		return
	}

	if !cfg.Enabled || cfg.Host == "" {
		return
	}

	msg := BuildMultipartMessage(cfg.FromName, cfg.FromEmail, strings.Join(job.Recipients, ", "), job.Subject, job.TextBody, job.HTMLBody)
	if err := SendMail(ctx, cfg, job.Recipients, msg); err != nil {
		log.Printf("[MAILER ERROR] Failed to dispatch email to %v (subject: %s): %v", job.Recipients, job.Subject, err)
	}
}

// SendAsync enqueues an email job to the in-memory background worker.
func (m *Mailer) SendAsync(job EmailJob) {
	if len(job.Recipients) == 0 {
		return
	}
	select {
	case m.jobs <- job:
	default:
		log.Printf("[MAILER WARN] Outbound email buffer full, dropping email to %v", job.Recipients)
	}
}

// SendDirect synchronously dispatches an email, useful for connection testing.
func (m *Mailer) SendDirect(ctx context.Context, recipients []string, subject, htmlBody, textBody string, cfgOverride *Config) error {
	cfg := cfgOverride
	if cfg == nil {
		var err error
		cfg, err = LoadConfig(ctx, m.db)
		if err != nil {
			return err
		}
	}

	msg := BuildMultipartMessage(cfg.FromName, cfg.FromEmail, strings.Join(recipients, ", "), subject, textBody, htmlBody)
	return SendMail(ctx, cfg, recipients, msg)
}

// SendPasswordReset queues a password reset email.
func (m *Mailer) SendPasswordReset(recipient, username, resetURL string) {
	if recipient == "" {
		return
	}
	htmlBody, textBody := PasswordResetTemplate(username, resetURL)
	m.SendAsync(EmailJob{
		Recipients: []string{recipient},
		Subject:    "Reset Your Password - Bedrock Server Manager",
		HTMLBody:   htmlBody,
		TextBody:   textBody,
	})
}

// SendEmailVerification queues an account verification email.
func (m *Mailer) SendEmailVerification(recipient, username, verifyURL string) {
	if recipient == "" {
		return
	}
	htmlBody, textBody := EmailVerificationTemplate(username, verifyURL)
	m.SendAsync(EmailJob{
		Recipients: []string{recipient},
		Subject:    "Verify Your Email Address - Bedrock Server Manager",
		HTMLBody:   htmlBody,
		TextBody:   textBody,
	})
}

// SendWelcomeEmail queues a welcome email for newly activated users.
func (m *Mailer) SendWelcomeEmail(recipient, username, planName, dashboardURL string) {
	if recipient == "" {
		return
	}
	htmlBody, textBody := WelcomeEmailTemplate(username, planName, dashboardURL)
	m.SendAsync(EmailJob{
		Recipients: []string{recipient},
		Subject:    "Welcome to Bedrock Server Manager",
		HTMLBody:   htmlBody,
		TextBody:   textBody,
	})
}

// SendSystemAlert queues a notification email according to alert categories.
// category must be one of: "crash", "security", "resource".
func (m *Mailer) SendSystemAlert(ctx context.Context, category, alertType, alertTitle, alertDescription string, fields map[string]string, dashboardURL string) {
	cfg, err := LoadConfig(ctx, m.db)
	if err != nil || !cfg.Enabled || cfg.Host == "" {
		return
	}

	switch category {
	case "crash":
		if !cfg.AlertCrashes {
			return
		}
	case "security":
		if !cfg.AlertSecurity {
			return
		}
	case "resource":
		if !cfg.AlertResources {
			return
		}
	}

	var recipients []string
	if cfg.AlertRecipients != "" {
		parts := strings.Split(cfg.AlertRecipients, ",")
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				recipients = append(recipients, trimmed)
			}
		}
	}

	if len(recipients) == 0 {
		recipients, _ = m.db.GetAdminEmails(ctx)
	}

	if len(recipients) == 0 {
		return
	}

	htmlBody, textBody := SystemAlertTemplate(alertType, alertTitle, alertDescription, fields, dashboardURL)
	m.SendAsync(EmailJob{
		Recipients: recipients,
		Subject:    "[" + alertType + "] " + alertTitle + " - Bedrock Server Manager",
		HTMLBody:   htmlBody,
		TextBody:   textBody,
	})
}

// Close gracefully stops the worker pool.
func (m *Mailer) Close() {
	close(m.stopChan)
	m.wg.Wait()
}
