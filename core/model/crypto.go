package model

import (
	"fmt"

	"golang.org/x/crypto/sha3"
)

func Keccak256(data string) string {
	hasher := sha3.NewLegacyKeccak256()

	hasher.Write([]byte(data))

	hash := hasher.Sum(nil)

	return fmt.Sprintf("%x", hash)
}
