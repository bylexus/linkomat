package api

import "time"

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type User struct {
	ID           uint      `gorm:"primaryKey"`
	Username     string    `gorm:"uniqueIndex;not null"`
	PasswordHash string    `gorm:"not null"`
	Active       bool      `gorm:"not null;default:true"`
	CreatedAt    time.Time `gorm:"not null"`
}

type Token struct {
	ID        uint       `gorm:"primaryKey"`
	UserID    uint       `gorm:"not null;index"`
	Type      string     `gorm:"not null"`
	Value     string     `gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time  `gorm:"not null"`
	RevokedAt *time.Time `gorm:"index"`
	CreatedAt time.Time  `gorm:"not null"`
}

type SchemaVersion struct {
	Version int `gorm:"primaryKey"`
}

type Friendship struct {
	ID            string    `gorm:"primaryKey"`
	RequestedByID uint      `gorm:"not null;uniqueIndex:idx_friends_pair,priority:1"`
	RequestedBy   User      `gorm:"foreignKey:RequestedByID;constraint:OnDelete:CASCADE"`
	AcceptedByID  uint      `gorm:"not null;uniqueIndex:idx_friends_pair,priority:2"`
	AcceptedBy    User      `gorm:"foreignKey:AcceptedByID;constraint:OnDelete:CASCADE"`
	RequestedAt   time.Time `gorm:"not null"`
	AcceptedAt    *time.Time
	DeclinedAt    *time.Time
}

func (Friendship) TableName() string {
	return "friends"
}
