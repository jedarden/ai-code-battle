package main

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestGenerateID(t *testing.T) {
	id, err := generateID("b_", 4)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(id, "b_") {
		t.Errorf("id should start with 'b_': %s", id)
	}
	if len(id) != 10 { // b_ + 8 hex chars
		t.Errorf("id length should be 10: got %d (%s)", len(id), id)
	}

	// Should be unique
	id2, _ := generateID("b_", 4)
	if id == id2 {
		t.Errorf("IDs should be unique: %s == %s", id, id2)
	}
}

func TestGenerateSecret(t *testing.T) {
	secret, err := generateSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(secret) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("secret length should be 64: got %d", len(secret))
	}
	// Should be valid hex
	_, err = hex.DecodeString(secret)
	if err != nil {
		t.Errorf("secret should be valid hex: %v", err)
	}
}

func TestEncryptDecrypt(t *testing.T) {
	// Generate a 256-bit key
	key := strings.Repeat("ab", 32) // 64 hex chars = 32 bytes
	plaintext := "my-secret-value-12345"

	encrypted, err := encryptSecret(plaintext, key)
	if err != nil {
		t.Fatal(err)
	}

	if encrypted == plaintext {
		t.Error("encrypted should differ from plaintext")
	}

	decrypted, err := decryptSecret(encrypted, key)
	if err != nil {
		t.Fatal(err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptDecrypt_DifferentCiphertexts(t *testing.T) {
	key := strings.Repeat("cd", 32)
	plaintext := "same-value"

	enc1, _ := encryptSecret(plaintext, key)
	enc2, _ := encryptSecret(plaintext, key)

	// AES-GCM with random nonce should produce different ciphertexts
	if enc1 == enc2 {
		t.Error("same plaintext should produce different ciphertexts (random nonce)")
	}

	// Both should decrypt to the same value
	dec1, _ := decryptSecret(enc1, key)
	dec2, _ := decryptSecret(enc2, key)
	if dec1 != plaintext || dec2 != plaintext {
		t.Errorf("both should decrypt to %q: got %q, %q", plaintext, dec1, dec2)
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := strings.Repeat("ab", 32)
	key2 := strings.Repeat("cd", 32)
	plaintext := "secret"

	encrypted, _ := encryptSecret(plaintext, key1)

	_, err := decryptSecret(encrypted, key2)
	if err == nil {
		t.Error("decryption with wrong key should fail")
	}
}

func TestEncrypt_InvalidKeyLength(t *testing.T) {
	_, err := encryptSecret("test", "abcd") // too short
	if err == nil {
		t.Error("short key should produce error")
	}
}
