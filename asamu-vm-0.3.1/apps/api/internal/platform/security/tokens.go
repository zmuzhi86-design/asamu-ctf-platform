package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	Roles        []string `json:"roles"`
	Permissions  []string `json:"permissions"`
	TokenVersion int      `json:"ver"`
	jwt.RegisteredClaims
}
type TokenManager struct {
	issuer string
	secret []byte
	ttl    time.Duration
}

func NewTokenManager(issuer, secret string, ttl time.Duration) *TokenManager {
	return &TokenManager{issuer: issuer, secret: []byte(secret), ttl: ttl}
}
func (m *TokenManager) Issue(userID string, tokenVersion int, roles, permissions []string) (string, time.Time, error) {
	now := time.Now().UTC()
	expires := now.Add(m.ttl)
	claims := Claims{Roles: roles, Permissions: permissions, TokenVersion: tokenVersion, RegisteredClaims: jwt.RegisteredClaims{Issuer: m.issuer, Subject: userID, ID: uuid.NewString(), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expires)}}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	return signed, expires, err
}
func (m *TokenManager) Parse(raw string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(raw, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		return nil, errors.New("invalid access token")
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	return claims, nil
}
func RandomToken(bytes int) (string, error) {
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
func TokenHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}
func FlagHMAC(secret []byte, flag string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(flag))
	return mac.Sum(nil)
}
func VerifyFlag(secret, expected []byte, candidate string) bool {
	actual := FlagHMAC(secret, candidate)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
func Encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}
func Decrypt(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, data := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, data, nil)
}
