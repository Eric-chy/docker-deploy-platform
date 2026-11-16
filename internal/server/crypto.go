package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

const secretPrefix = "aes256gcm:v1:"

func EncryptSecret(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
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
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return secretPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

func DecryptSecret(key []byte, value string) (string, error) {
	if len(value) <= len(secretPrefix) || value[:len(secretPrefix)] != secretPrefix {
		return "", errors.New("invalid encrypted secret")
	}
	encoded := value[len(secretPrefix):]
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("invalid encrypted secret")
	}
	plaintext, err := gcm.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
