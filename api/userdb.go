package api

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bylexus/linkomat/shared"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const userDirName = "users"

type Entry struct {
	ID        string     `gorm:"primaryKey;size:64"`
	Title     string     `gorm:"not null;default:''"`
	URL       string     `gorm:"not null;default:''"`
	Note      string     `gorm:"not null;default:''"`
	Tags      string     `gorm:"not null;default:[];type:text"`
	Meta      string     `gorm:"not null;default:'{}';type:text"`
	CreatedAt time.Time  `gorm:"not null"`
	UpdatedAt time.Time  `gorm:"not null;index"`
	DeletedAt *time.Time `gorm:"index"`
}

type PendingEntry struct {
	ID            string    `gorm:"primaryKey;size:64"`
	FromUsername  string    `gorm:"not null"`
	SourceEntryID string    `gorm:"not null"`
	Title         string    `gorm:"not null;default:''"`
	URL           string    `gorm:"not null;default:''"`
	Note          string    `gorm:"not null;default:''"`
	Tags          string    `gorm:"not null;default:[];type:text"`
	SharedAt      time.Time `gorm:"not null;index"`
}

type EntrySchemaVersion struct {
	Version int `gorm:"primaryKey"`
}

type UserDB struct {
	*gorm.DB
	username string
	path     string
}

type UserDBs struct {
	mu  sync.Mutex
	dbs map[string]*UserDB
	cfg *Config
}

func NewUserDBs(cfg *Config) *UserDBs {
	return &UserDBs{cfg: cfg, dbs: make(map[string]*UserDB)}
}

func (u *UserDBs) For(username string) (*UserDB, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if db, ok := u.dbs[username]; ok {
		return db, nil
	}
	db, err := OpenUserDB(u.cfg, username)
	if err != nil {
		return nil, err
	}
	u.dbs[username] = db
	return db, nil
}

func (u *UserDBs) Close() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	var firstErr error
	for _, db := range u.dbs {
		if err := db.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	u.dbs = make(map[string]*UserDB)
	return firstErr
}

func OpenUserDB(cfg *Config, username string) (*UserDB, error) {
	if username == "" {
		return nil, errors.New("username is required")
	}
	dir := filepath.Join(cfg.DataDir, userDirName)
	if err := os.MkdirAll(dir, metaDirPerm); err != nil {
		return nil, fmt.Errorf("create user db dir: %w", err)
	}
	dbPath := filepath.Join(dir, username+".db")
	dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on"
	gormCfg := &gorm.Config{
		Logger: logger.New(slogGormWriter{}, logger.Config{
			LogLevel: logger.Warn,
		}),
	}
	gdb, err := gorm.Open(sqlite.Open(dsn), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("open user db: %w", err)
	}
	ud := &UserDB{DB: gdb, username: username, path: dbPath}
	if err := ud.migrate(); err != nil {
		_ = ud.Close()
		return nil, fmt.Errorf("migrate user db: %w", err)
	}
	return ud, nil
}

func (u *UserDB) migrate() error {
	if err := u.DB.AutoMigrate(&Entry{}, &PendingEntry{}, &EntrySchemaVersion{}); err != nil {
		return err
	}
	var sv EntrySchemaVersion
	err := u.DB.First(&sv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return u.DB.Create(&EntrySchemaVersion{Version: shared.CurrentSchemaVersion}).Error
	}
	if err != nil {
		return err
	}
	if sv.Version < shared.CurrentSchemaVersion {
		return u.DB.Save(&EntrySchemaVersion{Version: shared.CurrentSchemaVersion}).Error
	}
	return nil
}

func (u *UserDB) Close() error {
	sqlDB, err := u.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
