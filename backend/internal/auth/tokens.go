// Package auth — генерация и хэширование токенов доступа.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/hex"
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewToken возвращает криптостойкий токен (256 бит энтропии, base32 без паддинга).
func NewToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return b32.EncodeToString(buf), nil
}

// Hash — SHA-256 хэш токена для хранения в БД (токены в открытом виде не храним).
func Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Match — сравнение токена с хэшем в константном времени.
func Match(token, hash string) bool {
	return subtle.ConstantTimeCompare([]byte(Hash(token)), []byte(hash)) == 1
}
