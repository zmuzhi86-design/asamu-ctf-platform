package security

import (
	"bytes"
	"testing"
	"time"
)

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "correct-horse-battery-staple") {
		t.Fatal("password should verify")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatal("wrong password verified")
	}
}
func TestTokenManager(t *testing.T) {
	manager := NewTokenManager("test", string(bytes.Repeat([]byte{'s'}, 32)), time.Minute)
	token, _, err := manager.Issue("4ce06ce5-f641-43c7-bc62-436f9d151d1c", 3, []string{"user"}, []string{"challenge.read"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := manager.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "4ce06ce5-f641-43c7-bc62-436f9d151d1c" {
		t.Fatalf("unexpected subject %s", claims.Subject)
	}
	if claims.TokenVersion != 3 {
		t.Fatalf("unexpected token version %d", claims.TokenVersion)
	}
}
func TestFlagHMACAndEncryption(t *testing.T) {
	secret := bytes.Repeat([]byte{'h'}, 32)
	flag := "flag{dynamic-secret}"
	digest := FlagHMAC(secret, flag)
	if !VerifyFlag(secret, digest, flag) {
		t.Fatal("valid flag rejected")
	}
	if VerifyFlag(secret, digest, "flag{other}") {
		t.Fatal("invalid flag accepted")
	}
	key := bytes.Repeat([]byte{'k'}, 32)
	ciphertext, err := Encrypt([]byte(flag), key)
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(plaintext) != flag {
		t.Fatal("decrypted flag mismatch")
	}
}
