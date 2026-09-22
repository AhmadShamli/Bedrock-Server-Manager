package auth

import (
	"testing"
	"time"
)

func TestPasswordHashing(t *testing.T) {
	pw := "Secret12345!"
	hash, err := HashPassword(pw, 8)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if !CheckPassword(hash, pw) {
		t.Errorf("expected CheckPassword to return true for correct password")
	}

	if CheckPassword(hash, "wrongpassword") {
		t.Errorf("expected CheckPassword to return false for incorrect password")
	}

	// Short password
	if _, err := HashPassword("short", 8); err != ErrPasswordTooShort {
		t.Errorf("expected ErrPasswordTooShort, got %v", err)
	}
}

func TestAccessKeyHMAC(t *testing.T) {
	pepper := "server_secret_pepper_123"
	key := "diamond-pickaxe"

	hash := HashAccessKey(pepper, key)
	if hash == "" {
		t.Fatalf("HashAccessKey returned empty string")
	}

	if !VerifyAccessKey(pepper, key, hash) {
		t.Errorf("expected VerifyAccessKey to succeed")
	}

	if VerifyAccessKey(pepper, "wrong-key", hash) {
		t.Errorf("expected VerifyAccessKey to fail with wrong key")
	}

	if VerifyAccessKey("wrong-pepper", key, hash) {
		t.Errorf("expected VerifyAccessKey to fail with wrong pepper")
	}
}

func TestJWT(t *testing.T) {
	secret := []byte("super-secret-jwt-signing-key-32bytes!")
	tokenStr, err := GenerateJWT(secret, 42, "admin", "admin", 1*time.Hour)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	claims, err := ValidateJWT(secret, tokenStr)
	if err != nil {
		t.Fatalf("ValidateJWT failed: %v", err)
	}

	if claims.UserID != 42 || claims.Username != "admin" || claims.Role != "admin" {
		t.Errorf("unexpected claims: %+v", claims)
	}

	// Tampered / wrong secret
	if _, err := ValidateJWT([]byte("wrong-secret-key-32bytes-padding!"), tokenStr); err == nil {
		t.Errorf("expected validation to fail with wrong secret")
	}
}

func TestTwoTierRateLimiter(t *testing.T) {
	now := time.Now()
	rl := NewRateLimiter(3, 5*time.Minute, 3, 5*time.Minute, 15*time.Minute)

	ip1 := "192.0.2.1"
	ip2 := "192.0.2.2"
	ip3 := "192.0.2.3"

	// IP1 fails 3 times
	for i := 0; i < 2; i++ {
		tripped := rl.RecordKnockFailure(ip1, now)
		if tripped {
			t.Errorf("unexpected circuit breaker trip on attempt %d", i+1)
		}
		res := rl.CheckKnockAttempt(ip1, now)
		if res.Blocked {
			t.Errorf("expected IP1 not to be blocked before threshold")
		}
	}

	// 3rd failure trips Level 1
	rl.RecordKnockFailure(ip1, now)
	res := rl.CheckKnockAttempt(ip1, now)
	if !res.Blocked || res.Level != 1 {
		t.Errorf("expected Level 1 block on 3rd failure, got %+v", res)
	}

	// IP2 should still be allowed
	res2 := rl.CheckKnockAttempt(ip2, now)
	if res2.Blocked {
		t.Errorf("expected IP2 to be allowed")
	}

	// Now fail IP2 and IP3 to trip Level 2 distributed breaker (threshold = 3 distinct IPs)
	rl.RecordKnockFailure(ip2, now)
	tripped := rl.RecordKnockFailure(ip3, now)
	if !tripped {
		t.Errorf("expected distributed circuit breaker to trip on 3rd distinct failed IP")
	}

	// Now even a clean IP4 should be blocked at Level 2
	ip4 := "192.0.2.4"
	res4 := rl.CheckKnockAttempt(ip4, now)
	if !res4.Blocked || res4.Level != 2 {
		t.Errorf("expected Level 2 circuit breaker block for IP4, got %+v", res4)
	}

	// Reset circuit breaker
	rl.ResetCircuitBreaker()
	res4After := rl.CheckKnockAttempt(ip4, now)
	if res4After.Blocked {
		t.Errorf("expected IP4 to be allowed after circuit breaker reset")
	}
}
