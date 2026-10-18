package database

import (
	"context"
	"testing"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

func TestAuthTokensAndEmailVerification(t *testing.T) {
	db, err := OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// 1. Create users: one admin with email, one user unverified, one user verified
	admin, err := db.CreateUserExtended(ctx, "admin_user", "admin@example.com", "hash1", models.RoleAdmin, nil, "active", nil, true)
	if err != nil {
		t.Fatalf("failed to create admin: %v", err)
	}
	if !admin.EmailVerified {
		t.Errorf("expected admin to be email verified")
	}

	userUnverified, err := db.CreateUserExtended(ctx, "regular_user", "user@example.com", "hash2", models.RoleUser, nil, "active", nil, false)
	if err != nil {
		t.Fatalf("failed to create regular user: %v", err)
	}
	if userUnverified.EmailVerified {
		t.Errorf("expected regular user to NOT be email verified")
	}

	// 2. Test GetAdminEmails
	adminEmails, err := db.GetAdminEmails(ctx)
	if err != nil {
		t.Fatalf("GetAdminEmails failed: %v", err)
	}
	if len(adminEmails) != 1 || adminEmails[0] != "admin@example.com" {
		t.Errorf("expected [admin@example.com], got %v", adminEmails)
	}

	// 3. Test GetUserByEmail and GetUserByUsernameOrEmail
	byEmail, err := db.GetUserByEmail(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if byEmail.ID != userUnverified.ID || byEmail.EmailVerified != false {
		t.Errorf("unexpected user returned: %+v", byEmail)
	}

	byUsernameOrEmail1, err := db.GetUserByUsernameOrEmail(ctx, "regular_user")
	if err != nil || byUsernameOrEmail1.ID != userUnverified.ID {
		t.Errorf("GetUserByUsernameOrEmail by username failed: %v", err)
	}

	byUsernameOrEmail2, err := db.GetUserByUsernameOrEmail(ctx, "user@example.com")
	if err != nil || byUsernameOrEmail2.ID != userUnverified.ID {
		t.Errorf("GetUserByUsernameOrEmail by email failed: %v", err)
	}

	// 4. Test SetUserEmailVerified
	if err := db.SetUserEmailVerified(ctx, userUnverified.ID, true); err != nil {
		t.Fatalf("SetUserEmailVerified failed: %v", err)
	}
	refetched, err := db.GetUserByID(ctx, userUnverified.ID)
	if err != nil || !refetched.EmailVerified {
		t.Errorf("expected user to be verified after SetUserEmailVerified, got: %+v", refetched)
	}

	// 5. Test AuthTokens: Create, Get, Consume
	tokenHash := "mock_sha256_hash_value_123456"
	expiresAt := time.Now().UTC().Add(15 * time.Minute)

	if err := db.CreateAuthToken(ctx, userUnverified.ID, "password_reset", tokenHash, expiresAt); err != nil {
		t.Fatalf("CreateAuthToken failed: %v", err)
	}

	tok, err := db.GetAuthTokenByHash(ctx, tokenHash, "password_reset")
	if err != nil {
		t.Fatalf("GetAuthTokenByHash failed: %v", err)
	}
	if tok.UserID != userUnverified.ID || tok.ConsumedAt != nil {
		t.Errorf("unexpected token state: %+v", tok)
	}

	// Consume token
	if err := db.ConsumeAuthToken(ctx, tokenHash); err != nil {
		t.Fatalf("ConsumeAuthToken failed: %v", err)
	}

	// Second consume should fail (already consumed)
	if err := db.ConsumeAuthToken(ctx, tokenHash); err == nil {
		t.Errorf("expected second ConsumeAuthToken to fail, but it succeeded")
	}

	// 6. Test InvalidateUserTokens
	tokenHash2 := "mock_hash_2"
	if err := db.CreateAuthToken(ctx, userUnverified.ID, "email_verification", tokenHash2, expiresAt); err != nil {
		t.Fatalf("CreateAuthToken 2 failed: %v", err)
	}
	if err := db.InvalidateUserTokens(ctx, userUnverified.ID, "email_verification"); err != nil {
		t.Fatalf("InvalidateUserTokens failed: %v", err)
	}
	tok2, err := db.GetAuthTokenByHash(ctx, tokenHash2, "email_verification")
	if err != nil {
		t.Fatalf("GetAuthTokenByHash failed: %v", err)
	}
	if tok2.ConsumedAt == nil {
		t.Errorf("expected token 2 to be invalidated/consumed")
	}
}
