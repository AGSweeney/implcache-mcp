// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package usage implements local usage analytics in a separate SQLite database.
package usage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

const (
	metaEnabled           = "enabled"
	metaRetentionDays     = "retention_days"
	metaStoreTaskText     = "store_task_text"
	metaStoreEvidenceText = "store_evidence_text"
	metaInstallSalt       = "install_salt"
	metaSchemaNote        = "schema_note"
)

// Store is the usage analytics database.
type Store struct {
	db     *sql.DB
	path   string
	mu     sync.RWMutex
	cfg    Config
	writer *asyncWriter
	closed atomic.Bool
	drops  atomic.Int64
}

// OpenOptions configures usage DB open.
type OpenOptions struct {
	Config Config
}

// DefaultUsageDBPath returns <dir(knowledgeDB)>/implcache-usage.db.
func DefaultUsageDBPath(knowledgeDB string) string {
	dir := filepath.Dir(knowledgeDB)
	if dir == "" || dir == "." {
		return "implcache-usage.db"
	}
	return filepath.Join(dir, "implcache-usage.db")
}

// Open opens or creates the usage database. If cfg.CLIDisabled, returns a
// disabled Store that does not open SQLite for writes (nil db).
func Open(path string, cfg Config) (*Store, error) {
	cfg = normalizeConfig(cfg, path)
	s := &Store{path: cfg.DBPath, cfg: cfg}
	if cfg.CLIDisabled {
		s.cfg.Enabled = false
		return s, nil
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil && filepath.Dir(cfg.DBPath) != "." {
		// ignore when dir is current
	}
	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=temp_store(MEMORY)",
		filepath.ToSlash(cfg.DBPath),
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open usage db: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	for _, pragma := range []string{
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA foreign_keys = ON`,
		`PRAGMA journal_mode = WAL`,
		`PRAGMA synchronous = NORMAL`,
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}
	if err := ensureSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	s.db = db
	if err := s.ensureInstallSalt(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.loadPersistedConfig(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	// CLI/env initial values seed meta on first open when missing.
	_ = s.seedMetaDefaults(context.Background(), cfg)
	_ = s.loadPersistedConfig(context.Background())
	s.writer = newAsyncWriter(s)
	s.writer.start()
	go s.retentionLoop()
	_ = s.PurgeExpired(context.Background())
	return s, nil
}

func normalizeConfig(cfg Config, path string) Config {
	if strings.TrimSpace(path) != "" {
		cfg.DBPath = path
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "implcache-usage.db"
	}
	if cfg.RetentionDays < 0 {
		cfg.RetentionDays = 90
	}
	if cfg.CLIDisabled {
		cfg.Enabled = false
	} else if !cfg.Enabled {
		// Default on when telemetry mode is local (callers omit Enabled=false).
		cfg.Enabled = true
	}
	return cfg
}

func ensureSchema(db *sql.DB) error {
	var ver int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&ver); err != nil {
		return err
	}
	if ver == SchemaVersion {
		return nil
	}
	if ver == 0 {
		if _, err := db.Exec(schemaSQL); err != nil {
			return fmt.Errorf("create usage schema: %w", err)
		}
		if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, SchemaVersion)); err != nil {
			return err
		}
		_, _ = db.Exec(`INSERT OR REPLACE INTO usage_meta(key,value) VALUES(?,?)`, metaSchemaNote, "usage analytics v2")
		return nil
	}
	if ver == 1 {
		if err := migrateUsageV1ToV2(db); err != nil {
			return err
		}
		if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, SchemaVersion)); err != nil {
			return err
		}
		_, _ = db.Exec(`INSERT OR REPLACE INTO usage_meta(key,value) VALUES(?,?)`, metaSchemaNote, "usage analytics v2")
		return nil
	}
	return fmt.Errorf("usage db schema version %d incompatible (want %d); delete the usage database file", ver, SchemaVersion)
}

func migrateUsageV1ToV2(db *sql.DB) error {
	alters := []string{
		`ALTER TABLE request_events ADD COLUMN returned_tokens INTEGER`,
		`ALTER TABLE request_events ADD COLUMN structured_tokens INTEGER`,
		`ALTER TABLE request_events ADD COLUMN raw_document_tokens INTEGER`,
		`ALTER TABLE request_events ADD COLUMN token_estimator_version TEXT`,
		`ALTER TABLE request_events ADD COLUMN coverage_applicable INTEGER`,
		`ALTER TABLE request_events ADD COLUMN request_class TEXT`,
	}
	for _, q := range alters {
		if _, err := db.Exec(q); err != nil {
			// Column may already exist on partial upgrade.
			if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
				return fmt.Errorf("migrate v2: %w", err)
			}
		}
	}
	if _, err := db.Exec(`
		UPDATE request_events
		SET returned_tokens = estimated_tokens
		WHERE returned_tokens IS NULL AND estimated_tokens IS NOT NULL`); err != nil {
		return err
	}
	for _, idx := range []string{
		`CREATE INDEX IF NOT EXISTS idx_request_events_time_status ON request_events(occurred_at, result_status)`,
		`CREATE INDEX IF NOT EXISTS idx_request_events_time_coverage ON request_events(occurred_at, coverage)`,
		`CREATE INDEX IF NOT EXISTS idx_request_events_class ON request_events(request_class)`,
		`CREATE INDEX IF NOT EXISTS idx_evidence_events_type ON evidence_events(evidence_type)`,
	} {
		if _, err := db.Exec(idx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ensureInstallSalt(ctx context.Context) error {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM usage_meta WHERE key=?`, metaInstallSalt).Scan(&v)
	if err == nil && v != "" {
		return nil
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	salt := hex.EncodeToString(b)
	_, err = s.db.ExecContext(ctx, `INSERT OR REPLACE INTO usage_meta(key,value) VALUES(?,?)`, metaInstallSalt, salt)
	return err
}

func (s *Store) seedMetaDefaults(ctx context.Context, cfg Config) error {
	setIfMissing := func(key, val string) {
		var existing string
		err := s.db.QueryRowContext(ctx, `SELECT value FROM usage_meta WHERE key=?`, key).Scan(&existing)
		if err == sql.ErrNoRows {
			_, _ = s.db.ExecContext(ctx, `INSERT INTO usage_meta(key,value) VALUES(?,?)`, key, val)
		}
	}
	en := "0"
	if cfg.Enabled && !cfg.CLIDisabled {
		en = "1"
	}
	setIfMissing(metaEnabled, en)
	days := cfg.RetentionDays
	if days < 0 {
		days = 90
	}
	setIfMissing(metaRetentionDays, strconv.Itoa(days))
	setIfMissing(metaStoreTaskText, boolStr(cfg.StoreTaskText))
	setIfMissing(metaStoreEvidenceText, boolStr(cfg.StoreEvidenceText))
	return nil
}

func (s *Store) loadPersistedConfig(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	get := func(key string) string {
		var v string
		_ = s.db.QueryRowContext(ctx, `SELECT value FROM usage_meta WHERE key=?`, key).Scan(&v)
		return v
	}
	if !s.cfg.CLIDisabled {
		s.cfg.Enabled = get(metaEnabled) != "0"
	} else {
		s.cfg.Enabled = false
	}
	if d, err := strconv.Atoi(get(metaRetentionDays)); err == nil {
		s.cfg.RetentionDays = d
	}
	s.cfg.StoreTaskText = get(metaStoreTaskText) == "1"
	s.cfg.StoreEvidenceText = get(metaStoreEvidenceText) == "1"
	return nil
}

// Close stops the writer and closes the DB.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.closed.Store(true)
	if s.writer != nil {
		s.writer.stop(time.Second)
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Config returns a copy of the current config.
func (s *Store) Config() Config {
	if s == nil {
		return Config{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// InstallSalt returns the installation-local salt for session hashing.
func (s *Store) InstallSalt(ctx context.Context) string {
	if s == nil || s.db == nil {
		return ""
	}
	var v string
	_ = s.db.QueryRowContext(ctx, `SELECT value FROM usage_meta WHERE key=?`, metaInstallSalt).Scan(&v)
	return v
}

// UpdateSettings persists runtime analytics settings (CLI disable still wins).
func (s *Store) UpdateSettings(ctx context.Context, enabled bool, retentionDays int, storeTask, storeEvidence bool) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("analytics database unavailable")
	}
	if s.cfg.CLIDisabled {
		return fmt.Errorf("analytics disabled by -telemetry=off")
	}
	if retentionDays < 0 {
		retentionDays = 90
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	puts := [][2]string{
		{metaEnabled, boolStr(enabled)},
		{metaRetentionDays, strconv.Itoa(retentionDays)},
		{metaStoreTaskText, boolStr(storeTask)},
		{metaStoreEvidenceText, boolStr(storeEvidence)},
	}
	for _, p := range puts {
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO usage_meta(key,value) VALUES(?,?)`, p[0], p[1]); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.mu.Lock()
	s.cfg.Enabled = enabled
	s.cfg.RetentionDays = retentionDays
	s.cfg.StoreTaskText = storeTask
	s.cfg.StoreEvidenceText = storeEvidence
	s.mu.Unlock()
	return nil
}

// Enabled reports whether new events should be recorded.
func (s *Store) Enabled() bool {
	if s == nil || s.closed.Load() || s.db == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Enabled && !s.cfg.CLIDisabled
}

// Status builds the public status payload.
func (s *Store) Status(ctx context.Context) Status {
	st := Status{
		LocalOnly:    true,
		MetadataOnly: true,
		Available:    false,
	}
	if s == nil {
		st.Message = "Analytics unavailable"
		return st
	}
	cfg := s.Config()
	st.Enabled = cfg.Enabled && !cfg.CLIDisabled
	st.DBPath = cfg.DBPath
	st.RetentionDays = cfg.RetentionDays
	st.StoreTaskText = cfg.StoreTaskText
	st.StoreEvidenceText = cfg.StoreEvidenceText
	st.DroppedEvents = s.drops.Load()
	if cfg.CLIDisabled {
		st.Enabled = false
		st.Message = "Local analytics disabled via -telemetry=off"
		return st
	}
	if s.db == nil {
		st.Message = "Analytics unavailable"
		return st
	}
	st.Available = true
	st.SchemaVersion = SchemaVersion
	st.TokenEstimatorVersion = TokenEstimatorVersion
	if !st.Enabled {
		st.Message = "Local analytics disabled. No new usage data is being recorded."
	} else {
		st.Message = "Local analytics enabled. Metadata only. No data leaves this machine."
		st.MetadataOnly = !cfg.StoreTaskText && !cfg.StoreEvidenceText
	}
	if fi, err := os.Stat(cfg.DBPath); err == nil {
		st.DatabaseBytes = fi.Size()
	}
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM request_events`).Scan(&st.RequestCount)
	var oldest, newest sql.NullString
	_ = s.db.QueryRowContext(ctx, `SELECT MIN(occurred_at), MAX(occurred_at) FROM request_events`).Scan(&oldest, &newest)
	if oldest.Valid {
		st.OldestAt = oldest.String
	}
	if newest.Valid {
		st.NewestAt = newest.String
	}
	return st
}

// ClearAll deletes all analytics events (keeps meta/salt).
func (s *Store) ClearAll(ctx context.Context, vacuum bool) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("analytics database unavailable")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, t := range []string{"outcome_events", "evidence_events", "retrieval_events", "request_roots", "request_events"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+t); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if vacuum {
		_, _ = s.db.ExecContext(ctx, `VACUUM`)
	}
	return nil
}

// PurgeExpired removes events older than retention.
func (s *Store) PurgeExpired(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	cfg := s.Config()
	if cfg.RetentionDays <= 0 {
		return nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -cfg.RetentionDays).Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT request_id FROM request_events WHERE occurred_at < ?`, cutoff)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		for _, q := range []string{
			`DELETE FROM evidence_events WHERE request_id=?`,
			`DELETE FROM retrieval_events WHERE request_id=?`,
			`DELETE FROM request_roots WHERE request_id=?`,
			`DELETE FROM outcome_events WHERE request_id=?`,
			`DELETE FROM request_events WHERE request_id=?`,
		} {
			if _, err := tx.ExecContext(ctx, q, id); err != nil {
				return err
			}
		}
	}
	_, _ = tx.ExecContext(ctx, `DELETE FROM outcome_events WHERE occurred_at < ?`, cutoff)
	return tx.Commit()
}

func (s *Store) retentionLoop() {
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for range t.C {
		if s.closed.Load() {
			return
		}
		_ = s.PurgeExpired(context.Background())
	}
}

func boolStr(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func (s *Store) incDrops() { s.drops.Add(1) }
