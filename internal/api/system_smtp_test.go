package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/mail"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

func TestSystemSMTPMaskingAndTest(t *testing.T) {
	db, err := database.OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// 1. Set SMTP password in DB
	_ = db.SetSetting(ctx, mail.SettingSMTPPassword, "super_secret_smtp_pass")

	rateLimiter := auth.NewRateLimiter(5, 5*time.Minute, 10, 5*time.Minute, 15*time.Minute)
	systemHandler := NewSystemHandler(db, "", nil, nil, nil, nil, rateLimiter)
	mailer := mail.NewMailer(db)
	defer mailer.Close()
	systemHandler.SetMailer(mailer)

	// 2. GetSettings should redact smtp_password to ******
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/system/settings", nil)
	wGet := httptest.NewRecorder()
	systemHandler.GetSettings(wGet, reqGet)

	if wGet.Code != http.StatusOK {
		t.Fatalf("GetSettings failed: %d", wGet.Code)
	}
	var settings map[string]string
	_ = json.Unmarshal(wGet.Body.Bytes(), &settings)
	if settings[mail.SettingSMTPPassword] != "******" {
		t.Errorf("expected smtp_password to be masked with '******', got: %s", settings[mail.SettingSMTPPassword])
	}

	// 3. UpdateSetting with ****** should ignore and preserve existing password
	updatePayload, _ := json.Marshal(map[string]string{
		"key":   mail.SettingSMTPPassword,
		"value": "******",
	})
	reqUpdate := httptest.NewRequest(http.MethodPost, "/api/v1/system/settings", bytes.NewReader(updatePayload))
	wUpdate := httptest.NewRecorder()
	systemHandler.UpdateSetting(wUpdate, reqUpdate)

	storedPass, _ := db.GetSetting(ctx, mail.SettingSMTPPassword)
	if storedPass != "super_secret_smtp_pass" {
		t.Errorf("expected existing password to remain 'super_secret_smtp_pass', got: %s", storedPass)
	}

	// 4. Update with actual new password
	updateReal, _ := json.Marshal(map[string]string{
		"key":   mail.SettingSMTPPassword,
		"value": "new_real_pass",
	})
	reqReal := httptest.NewRequest(http.MethodPost, "/api/v1/system/settings", bytes.NewReader(updateReal))
	wReal := httptest.NewRecorder()
	systemHandler.UpdateSetting(wReal, reqReal)

	storedPass2, _ := db.GetSetting(ctx, mail.SettingSMTPPassword)
	if storedPass2 != "new_real_pass" {
		t.Errorf("expected updated password 'new_real_pass', got: %s", storedPass2)
	}

	// 5. Test TestSMTP missing recipient check
	testReqNoRecipient, _ := json.Marshal(map[string]string{
		"host": "smtp.example.com",
	})
	reqTest1 := httptest.NewRequest(http.MethodPost, "/api/v1/system/smtp/test", bytes.NewReader(testReqNoRecipient))
	wTest1 := httptest.NewRecorder()
	systemHandler.TestSMTP(wTest1, reqTest1)

	if wTest1.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request when no recipient can be derived, got: %d", wTest1.Code)
	}

	// Create an admin with email
	_, _ = db.CreateUserExtended(ctx, "admin", "admin@example.com", "hash", models.RoleAdmin, nil, "active", nil, true)

	// Now TestSMTP with invalid host will fail with BadGateway (502)
	testReqWithHost, _ := json.Marshal(map[string]interface{}{
		"recipient":  "admin@example.com",
		"host":       "127.0.0.1",
		"port":       65530,
		"encryption": "none",
	})
	reqTest2 := httptest.NewRequest(http.MethodPost, "/api/v1/system/smtp/test", bytes.NewReader(testReqWithHost))
	wTest2 := httptest.NewRecorder()
	systemHandler.TestSMTP(wTest2, reqTest2)

	if wTest2.Code != http.StatusBadGateway {
		t.Errorf("expected 502 Bad Gateway for unreachable SMTP host, got: %d", wTest2.Code)
	}
}
