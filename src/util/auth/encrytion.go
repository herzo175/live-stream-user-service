package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"io"
)

// NOTE: do not use for passwords
func CreateHash(data []byte) string {
	hasher := md5.New()
	hasher.Write(data)
	return hex.EncodeToString(hasher.Sum(nil))
}

func CreateGCM(password []byte) (gcm cipher.AEAD, err error) {
	block, err := aes.NewCipher([]byte(CreateHash(password)))

	if err != nil {
		return gcm, err
	}

	return cipher.NewGCM(block)
}

func Encrpyt(data []byte, password []byte) (ciphertext []byte, err error) {
	gcm, err := CreateGCM(password)

	if err != nil {
		return ciphertext, err
	}

	nonce := make([]byte, gcm.NonceSize())

	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return ciphertext, nil
	}

	ciphertext = gcm.Seal(nonce, nonce, data, nil)

	return ciphertext, nil
}

func Decrypt(data []byte, password []byte) (plaintext []byte, err error) {
	gcm, err := CreateGCM(password)

	if err != nil {
		return plaintext, err
	}

	nonceSize := gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
