package gitclient

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

func EncryptSecret(secret []byte, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}

	block, err := aes.NewCipher(deriveKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	payload := gcm.Seal(nil, nonce, []byte(value), nil)
	sealed := append(nonce, payload...)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func DecryptSecret(secret []byte, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}

	encoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(deriveKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(encoded) < gcm.NonceSize() {
		return "", errors.New("invalid encrypted payload")
	}

	nonce := encoded[:gcm.NonceSize()]
	ciphertext := encoded[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func deriveKey(secret []byte) []byte {
	sum := sha256.Sum256(secret)
	return sum[:]
}
