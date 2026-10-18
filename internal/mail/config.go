package mail

import (
	"context"
	"strconv"
	"strings"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
)

// Setting keys stored in the database settings table.
const (
	SettingSMTPEnabled                 = "smtp_enabled"
	SettingSMTPHost                    = "smtp_host"
	SettingSMTPPort                    = "smtp_port"
	SettingSMTPEncryption              = "smtp_encryption" // "starttls", "ssl_tls", "none"
	SettingSMTPAuthType                = "smtp_auth_type"   // "plain", "login", "anonymous"
	SettingSMTPUsername                = "smtp_username"
	SettingSMTPPassword                = "smtp_password"
	SettingSMTPFromEmail               = "smtp_from_email"
	SettingSMTPFromName                = "smtp_from_name"
	SettingSMTPSkipTLSVerify           = "smtp_skip_tls_verify"
	SettingAppURL                      = "app_url"
	SettingSMTPRequireEmailVerify      = "smtp_require_email_verification"
	SettingSMTPSendWelcomeEmail        = "smtp_send_welcome_email"
	SettingSMTPAlertRecipients         = "smtp_alert_recipients"
	SettingSMTPAlertCrashes            = "smtp_alert_crashes"
	SettingSMTPAlertSecurity           = "smtp_alert_security"
	SettingSMTPAlertResources          = "smtp_alert_resources"
)

// Config represents runtime configuration for outgoing mail delivery.
type Config struct {
	Enabled                  bool   `json:"enabled"`
	Host                     string `json:"host"`
	Port                     int    `json:"port"`
	Encryption               string `json:"encryption"` // starttls, ssl_tls, none
	AuthType                 string `json:"auth_type"`   // plain, login, anonymous
	Username                 string `json:"username"`
	Password                 string `json:"password,omitempty"`
	FromEmail                string `json:"from_email"`
	FromName                 string `json:"from_name"`
	SkipTLSVerify            bool   `json:"skip_tls_verify"`
	AppURL                   string `json:"app_url"`
	RequireEmailVerification bool   `json:"require_email_verification"`
	SendWelcomeEmail         bool   `json:"send_welcome_email"`
	AlertRecipients          string `json:"alert_recipients"`
	AlertCrashes             bool   `json:"alert_crashes"`
	AlertSecurity            bool   `json:"alert_security"`
	AlertResources           bool   `json:"alert_resources"`
}

// LoadConfig loads the SMTP configuration from the database.
func LoadConfig(ctx context.Context, db *database.ManagerDB) (*Config, error) {
	settings, err := db.GetAllSettings(ctx)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Enabled:                  settings[SettingSMTPEnabled] == "true" || settings[SettingSMTPEnabled] == "1",
		Host:                     strings.TrimSpace(settings[SettingSMTPHost]),
		Port:                     587,
		Encryption:               "starttls",
		AuthType:                 "plain",
		Username:                 strings.TrimSpace(settings[SettingSMTPUsername]),
		Password:                 settings[SettingSMTPPassword],
		FromEmail:                strings.TrimSpace(settings[SettingSMTPFromEmail]),
		FromName:                 strings.TrimSpace(settings[SettingSMTPFromName]),
		SkipTLSVerify:            settings[SettingSMTPSkipTLSVerify] == "true" || settings[SettingSMTPSkipTLSVerify] == "1",
		AppURL:                   strings.TrimRight(strings.TrimSpace(settings[SettingAppURL]), "/"),
		RequireEmailVerification: settings[SettingSMTPRequireEmailVerify] == "true" || settings[SettingSMTPRequireEmailVerify] == "1",
		SendWelcomeEmail:         settings[SettingSMTPSendWelcomeEmail] == "true" || settings[SettingSMTPSendWelcomeEmail] == "1",
		AlertRecipients:          strings.TrimSpace(settings[SettingSMTPAlertRecipients]),
		AlertCrashes:             settings[SettingSMTPAlertCrashes] == "true" || settings[SettingSMTPAlertCrashes] == "1",
		AlertSecurity:            settings[SettingSMTPAlertSecurity] == "true" || settings[SettingSMTPAlertSecurity] == "1",
		AlertResources:           settings[SettingSMTPAlertResources] == "true" || settings[SettingSMTPAlertResources] == "1",
	}

	if p, err := strconv.Atoi(settings[SettingSMTPPort]); err == nil && p > 0 {
		cfg.Port = p
	}

	if enc := strings.ToLower(strings.TrimSpace(settings[SettingSMTPEncryption])); enc != "" {
		cfg.Encryption = enc
	}

	if authT := strings.ToLower(strings.TrimSpace(settings[SettingSMTPAuthType])); authT != "" {
		cfg.AuthType = authT
	}

	if cfg.FromName == "" {
		cfg.FromName = "Bedrock Server Manager"
	}

	return cfg, nil
}
