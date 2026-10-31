package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/bylexus/linkomat/shared"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const metaDirPerm = 0o700

type MetaDB struct {
	*gorm.DB
	cfg *Config
}

func OpenMetaDB(cfg *Config) (*MetaDB, error) {
	if err := os.MkdirAll(cfg.DataDir, metaDirPerm); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dbPath := filepath.Join(cfg.DataDir, "meta.db")
	dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on"
	gormCfg := &gorm.Config{
		Logger: logger.New(slogGormWriter{}, logger.Config{
			LogLevel: logger.Warn,
		}),
	}
	gdb, err := gorm.Open(sqlite.Open(dsn), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("open meta db: %w", err)
	}
	m := &MetaDB{DB: gdb, cfg: cfg}
	if err := m.migrate(); err != nil {
		return nil, fmt.Errorf("migrate meta db: %w", err)
	}
	return m, nil
}

func (m *MetaDB) migrate() error {
	if err := m.DB.AutoMigrate(&User{}, &Token{}, &Friendship{}, &SchemaVersion{}); err != nil {
		return err
	}
	var sv SchemaVersion
	err := m.DB.First(&sv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return m.DB.Create(&SchemaVersion{Version: shared.CurrentSchemaVersion}).Error
	}
	if err != nil {
		return err
	}
	if sv.Version < shared.CurrentSchemaVersion {
		return m.DB.Save(&SchemaVersion{Version: shared.CurrentSchemaVersion}).Error
	}
	return nil
}

func (m *MetaDB) Close() error {
	sqlDB, err := m.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

type slogGormWriter struct{}

func (slogGormWriter) Printf(format string, args ...interface{}) {
	slog.Warn(fmt.Sprintf(format, args...), slog.String("component", "gorm"))
}