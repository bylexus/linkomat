package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    uint32 = 1
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 2
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

func hashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return encoded, nil
}

func verifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("invalid argon2id hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("parse version: %w", err)
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false, fmt.Errorf("parse params: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want)))
	return constantTimeEq(got, want), nil
}

func constantTimeEq(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

func generateTokenValue() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func (m *MetaDB) IssueTokenPair(userID uint) (accessValue, refreshValue string, err error) {
	now := time.Now()
	access := Token{
		UserID:    userID,
		Type:      TokenTypeAccess,
		Value:     mustTokenValue(),
		ExpiresAt: now.Add(time.Duration(m.cfg.AccessTTL) * time.Second),
		CreatedAt: now,
	}
	refresh := Token{
		UserID:    userID,
		Type:      TokenTypeRefresh,
		Value:     mustTokenValue(),
		ExpiresAt: now.Add(time.Duration(m.cfg.RefreshTTL) * time.Second),
		CreatedAt: now,
	}
	if err := m.DB.Create(&access).Error; err != nil {
		return "", "", fmt.Errorf("create access token: %w", err)
	}
	if err := m.DB.Create(&refresh).Error; err != nil {
		return "", "", fmt.Errorf("create refresh token: %w", err)
	}
	return access.Value, refresh.Value, nil
}

func (m *MetaDB) FindActiveToken(value, tokenType string) (*Token, error) {
	var t Token
	err := m.DB.Where("value = ? AND type = ? AND revoked_at IS NULL AND expires_at > ?",
		value, tokenType, time.Now()).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (m *MetaDB) RevokeToken(value string) error {
	now := time.Now()
	return m.DB.Model(&Token{}).Where("value = ? AND revoked_at IS NULL", value).
		Update("revoked_at", &now).Error
}

func (m *MetaDB) RevokeAllUserTokens(userID uint) error {
	now := time.Now()
	return m.DB.Model(&Token{}).Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", &now).Error
}

func mustTokenValue() string {
	v, err := generateTokenValue()
	if err != nil {
		panic(err)
	}
	return v
}