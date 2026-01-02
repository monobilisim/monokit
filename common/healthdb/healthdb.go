package healthdb

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	sqlite "github.com/glebarez/sqlite"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var (
	dbOnce sync.Once
	dbInst *gorm.DB
)

// KVEntry is a simple key/value row scoped by module.
// Value V is typically JSON (TEXT), but the schema is agnostic.
type KVEntry struct {
	ID          uint       `gorm:"primaryKey"`
	Module      string     `gorm:"index:idx_module_key,unique"`
	K           string     `gorm:"index:idx_module_key,unique"`
	V           string     `gorm:"type:text"`
	CachedAt    time.Time  `gorm:"index"`
	NextCheckAt *time.Time `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// getDefaultDBPath chooses a persistent location when possible, otherwise falls back to tmp.
func getDefaultDBPath() string {
	if runtime.GOOS == "windows" {
		// Windows specific logic
		// Check environment first (similar to user mode check)
		// We can't easily check 'root' on Windows in same way, but let's try ProgramData first
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = "C:\\ProgramData"
		}
		sysPath := filepath.Join(programData, "mono")
		// Try to create/access system path
		if err := os.MkdirAll(sysPath, 0755); err == nil {
			return filepath.Join(sysPath, "health.db")
		}

		// Fallback to AppData (User mode)
		appData := os.Getenv("LOCALAPPDATA")
		if appData != "" {
			userPath := filepath.Join(appData, "monokit")
			_ = os.MkdirAll(userPath, 0755)
			return filepath.Join(userPath, "health.db")
		}

		// Fallback to Temp
		tmp := filepath.Join(os.TempDir(), "monokit")
		_ = os.MkdirAll(tmp, 0755)
		return filepath.Join(tmp, "health.db")
	}

	// Prefer XDG state dir for non-root users
	if os.Geteuid() != 0 {
		xdgState := os.Getenv("XDG_STATE_HOME")
		if xdgState == "" {
			home := os.Getenv("HOME")
			if home != "" {
				xdgState = filepath.Join(home, ".local", "state")
			}
		}
		if xdgState != "" {
			_ = os.MkdirAll(filepath.Join(xdgState, "mono"), 0o755)
			return filepath.Join(xdgState, "mono", "health.db")
		}
	}
	// Root or no HOME: use /var/lib/mono if possible
	if os.Geteuid() == 0 {
		if err := os.MkdirAll("/var/lib/mono", 0o755); err == nil {
			return "/var/lib/mono/health.db"
		}
	}
	// Fallback to tmp; ensure directory exists
	tmp := filepath.Join(os.TempDir(), "mono")
	_ = os.MkdirAll(tmp, 0o755)
	return filepath.Join(tmp, "health.db")
}

// Get returns the shared *gorm.DB instance (initializing it on first use).
func Get() *gorm.DB {
	dbOnce.Do(func() {
		path := getDefaultDBPath()
		gdb, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Error)})
		if err != nil {
			log.Fatal().Err(err).Str("path", path).Msg("healthdb: failed to open SQLite database")
		}
		if err := gdb.AutoMigrate(&KVEntry{}); err != nil {
			log.Fatal().Err(err).Msg("healthdb: failed to migrate schema")
		}
		dbInst = gdb
		log.Debug().Str("path", path).Msg("healthdb: initialized SQLite database")
	})
	return dbInst
}

// PutJSON stores a JSON string value under (module, key).
// cachedAt defaults to now if zero.
func PutJSON(module, key, json string, nextCheckAt *time.Time, cachedAt time.Time) error {
	if cachedAt.IsZero() {
		cachedAt = time.Now()
	}
	db := Get()
	// Avoid "record not found" logs by checking existence first
	var cnt int64
	if err := db.Model(&KVEntry{}).Where("module = ? AND k = ?", module, key).Count(&cnt).Error; err != nil {
		return fmt.Errorf("healthdb: count failed: %w", err)
	}
	if cnt > 0 {
		var existing KVEntry
		if err := db.Where("module = ? AND k = ?", module, key).First(&existing).Error; err != nil {
			return fmt.Errorf("healthdb: fetch failed: %w", err)
		}
		existing.V = json
		existing.CachedAt = cachedAt
		existing.NextCheckAt = nextCheckAt
		return db.Save(&existing).Error
	}
	entry := KVEntry{Module: module, K: key, V: json, CachedAt: cachedAt, NextCheckAt: nextCheckAt}
	return db.Create(&entry).Error
}

// GetJSON retrieves a JSON string value for (module, key).
func GetJSON(module, key string) (json string, cachedAt time.Time, nextCheckAt *time.Time, found bool, err error) {
	db := Get()
	// Avoid logging by not triggering a not-found error
	var cnt int64
	if err := db.Model(&KVEntry{}).Where("module = ? AND k = ?", module, key).Count(&cnt).Error; err != nil {
		return "", time.Time{}, nil, false, err
	}
	if cnt == 0 {
		return "", time.Time{}, nil, false, nil
	}
	var entry KVEntry
	if err := db.Where("module = ? AND k = ?", module, key).First(&entry).Error; err != nil {
		return "", time.Time{}, nil, false, err
	}
	return entry.V, entry.CachedAt, entry.NextCheckAt, true, nil
}

// Delete removes a key for a given module.
func Delete(module, key string) error {
	db := Get()
	return db.Where("module = ? AND k = ?", module, key).Delete(&KVEntry{}).Error
}
