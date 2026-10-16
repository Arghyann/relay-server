package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

func getMachineKey() ([]byte, error) {
	data, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		data, err = os.ReadFile("/var/lib/dbus/machine-id")
		if err != nil {
			return nil, fmt.Errorf("could not read machine-id: %w", err)
		}
	}
	hash := sha256.Sum256([]byte("relay-auth-salt:" + strings.TrimSpace(string(data))))
	return hash[:], nil
}

func encryptKey(plaintext string) ([]byte, error) {
	key, err := getMachineKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return aesGCM.Seal(nonce, nonce, []byte(strings.TrimSpace(plaintext)), nil), nil
}

func decryptKey(ciphertext []byte) (string, error) {
	key, err := getMachineKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, encryptedData := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := aesGCM.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (wrong machine or corrupted): %w", err)
	}

	return string(plaintext), nil
}

func getKeyFilePath() (string, string) {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		if u, err := user.Lookup(sudoUser); err == nil {
			dir := filepath.Join(u.HomeDir, ".config", "relay")
			return dir, filepath.Join(dir, "key.enc")
		}
		dir := filepath.Join("/home", sudoUser, ".config", "relay")
		return dir, filepath.Join(dir, "key.enc")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		dir := "/etc/relay"
		return dir, filepath.Join(dir, "key.enc")
	}
	dir := filepath.Join(home, ".config", "relay")
	return dir, filepath.Join(dir, "key.enc")
}

func resolveAPIKey(flagKey string) string {
	configDir, keyFile := getKeyFilePath()

	if flagKey != "" {
		encrypted, err := encryptKey(flagKey)
		if err != nil {
			log.Printf("Warning: failed to encrypt key: %v", err)
			return flagKey
		}

		_ = os.MkdirAll(configDir, 0700)
		if err := os.WriteFile(keyFile, encrypted, 0600); err == nil {
			log.Printf("Encrypted and saved API key to %s\n", keyFile)
			if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
				if u, err := user.Lookup(sudoUser); err == nil {
					uid, _ := strconv.Atoi(u.Uid)
					gid, _ := strconv.Atoi(u.Gid)
					_ = os.Chown(configDir, uid, gid)
					_ = os.Chown(keyFile, uid, gid)
				}
			}
		}
		return flagKey
	}

	if envKey := os.Getenv("RELAY_KEY"); envKey != "" {
		return envKey
	}

	data, err := os.ReadFile(keyFile)
	if err == nil && len(data) > 0 {
		decrypted, err := decryptKey(data)
		if err == nil && len(decrypted) > 0 {
			return decrypted
		}
		log.Printf("Warning: failed to decrypt %s: %v\n", keyFile, err)
	}

	log.Fatal("No API key provided. Run with -key <key> once to save it, or set RELAY_KEY.")
	return ""
}
