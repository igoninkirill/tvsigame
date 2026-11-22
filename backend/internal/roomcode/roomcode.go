// Package roomcode — генерация коротких кодов комнат.
// Алфавит исключает визуально неоднозначные символы (0/O, 1/I/L).
package roomcode

import (
	"crypto/rand"
	"math/big"
)

const alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// Generate возвращает код длиной length символов.
func Generate(length int) (string, error) {
	if length < 4 {
		length = 4
	}
	if length > 6 {
		length = 6
	}
	out := make([]byte, length)
	max := big.NewInt(int64(len(alphabet)))
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = alphabet[n.Int64()]
	}
	return string(out), nil
}
