package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrInvalidToken     = errors.New("invalid or expired token")
)

// Claims represents the JWT claims payload for authenticated users.
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// HashPassword hashes a user password using bcrypt.
func HashPassword(password string, minLen int) (string, error) {
	if len(password) < minLen {
		return "", ErrPasswordTooShort
	}
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword verifies a plaintext password against a bcrypt hash.
func CheckPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// HashAccessKey creates a fast keyed HMAC-SHA256 hash using the server pepper.
// Extremely lightweight (<0.01ms CPU, 0 MB RAM) to prevent CPU starvation attacks.
func HashAccessKey(pepper, passphrase string) string {
	mac := hmac.New(sha256.New, []byte(pepper))
	mac.Write([]byte(passphrase))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyAccessKey compares candidate passphrase HMAC against stored hash in constant time.
func VerifyAccessKey(pepper, candidatePassphrase, storedHash string) bool {
	candidateHash := HashAccessKey(pepper, candidatePassphrase)
	return subtle.ConstantTimeCompare([]byte(candidateHash), []byte(storedHash)) == 1
}

// GenerateRandomToken generates cryptographically secure hex tokens.
func GenerateRandomToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// HashToken generates a SHA256 hex hash of an opaque token for database lookups.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// GenerateJWT creates a signed JWT token for a user.
func GenerateJWT(secret []byte, userID int64, username, role string, duration time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Subject:   username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// ValidateJWT validates a signed JWT string and returns its claims.
func ValidateJWT(secret []byte, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}
