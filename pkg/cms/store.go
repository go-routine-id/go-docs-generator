// Package cms implements the authoring backend that sits on top of the
// docs-generator. It persists session state in SQLite and emits YAML to the
// directory docs-generator watches — the docs spec stays the source of truth.
package cms

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite connection and exposes the small surface the CMS
// needs (sessions for MVP, more tables as features land).
type Store struct {
	db *sql.DB
}

// OpenStore opens (creating if absent) the SQLite database at path and applies
// the embedded schema. Pass ":memory:" for tests.
func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	// SQLite default is a serialized single connection. WAL gives concurrent
	// reads while one writer; foreign_keys must be enabled per-connection.
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set pragmas: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error { return s.db.Close() }

// migrate runs idempotent CREATE TABLE IF NOT EXISTS for the current schema.
// When we need to evolve the schema we'll switch to a versioned migrator;
// for the MVP this is enough.
func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			token       TEXT PRIMARY KEY,
			user        TEXT NOT NULL,
			created_at  INTEGER NOT NULL,
			expires_at  INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS sessions_expires_idx ON sessions(expires_at)`,
		`CREATE TABLE IF NOT EXISTS drafts (
			file_path   TEXT NOT NULL,
			guide_id    TEXT NOT NULL,
			payload     TEXT NOT NULL,
			created_at  INTEGER NOT NULL,
			updated_at  INTEGER NOT NULL,
			PRIMARY KEY (file_path, guide_id)
		)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("migrate: %w (%s)", err, q)
		}
	}
	return nil
}

// SessionTTL is how long a fresh login stays valid before requiring a re-auth.
const SessionTTL = 12 * time.Hour

// Session represents a logged-in admin session.
type Session struct {
	Token     string
	User      string
	ExpiresAt time.Time
}

// ErrSessionNotFound is returned when a cookie token isn't in the table or has
// expired. Auth middleware treats this as "not logged in", not as an error.
var ErrSessionNotFound = errors.New("session not found or expired")

// NewSession mints a fresh random token, stores it with a TTL, and returns it.
// The caller writes the token into a Set-Cookie header on the response.
func (s *Store) NewSession(user string) (*Session, error) {
	tok, err := randomToken()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	exp := now.Add(SessionTTL)
	_, err = s.db.Exec(
		`INSERT INTO sessions (token, user, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		tok, user, now.Unix(), exp.Unix(),
	)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}
	return &Session{Token: tok, User: user, ExpiresAt: exp}, nil
}

// LookupSession returns the session for a token if it exists and hasn't
// expired. Expired rows are NOT deleted here — that's GarbageCollect's job.
func (s *Store) LookupSession(token string) (*Session, error) {
	row := s.db.QueryRow(
		`SELECT token, user, expires_at FROM sessions WHERE token = ? AND expires_at > ?`,
		token, time.Now().Unix(),
	)
	var sess Session
	var exp int64
	if err := row.Scan(&sess.Token, &sess.User, &exp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("scan session: %w", err)
	}
	sess.ExpiresAt = time.Unix(exp, 0)
	return &sess, nil
}

// DeleteSession removes a session by token (logout). Missing tokens are a no-op.
func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// GarbageCollect deletes expired sessions. Call periodically (we wire a
// goroutine in cms-server/main.go) so the table doesn't grow unbounded.
func (s *Store) GarbageCollect() error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, time.Now().Unix())
	return err
}

// randomToken returns 32 bytes of crypto-random hex. 256 bits of entropy is
// way more than enough — a guesser would have to win the lottery before noon.
func randomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("read random: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// Draft represents an unpublished, in-progress edit. Payload is an opaque
// JSON-encoded value the handler layer owns — store treats it as a blob.
type Draft struct {
	FilePath  string
	GuideID   string
	Payload   string
	UpdatedAt time.Time
}

// ErrDraftNotFound is returned by GetDraft when no draft exists for the
// (file, guide) pair. Callers treat this as "use the published YAML" rather
// than a real error.
var ErrDraftNotFound = errors.New("draft not found")

// UpsertDraft saves a draft for (filePath, guideID), creating or overwriting.
// One draft per guide — no version history in this MVP.
func (s *Store) UpsertDraft(filePath, guideID, payload string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(
		`INSERT INTO drafts (file_path, guide_id, payload, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(file_path, guide_id) DO UPDATE SET payload = excluded.payload, updated_at = excluded.updated_at`,
		filePath, guideID, payload, now, now,
	)
	if err != nil {
		return fmt.Errorf("upsert draft: %w", err)
	}
	return nil
}

// GetDraft returns the draft for (filePath, guideID) or ErrDraftNotFound.
func (s *Store) GetDraft(filePath, guideID string) (*Draft, error) {
	row := s.db.QueryRow(
		`SELECT file_path, guide_id, payload, updated_at FROM drafts WHERE file_path = ? AND guide_id = ?`,
		filePath, guideID,
	)
	var d Draft
	var updated int64
	if err := row.Scan(&d.FilePath, &d.GuideID, &d.Payload, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDraftNotFound
		}
		return nil, fmt.Errorf("scan draft: %w", err)
	}
	d.UpdatedAt = time.Unix(updated, 0)
	return &d, nil
}

// DeleteDraft removes a draft. Missing rows are a no-op so handlers can
// "ensure no draft" without first checking existence.
func (s *Store) DeleteDraft(filePath, guideID string) error {
	_, err := s.db.Exec(`DELETE FROM drafts WHERE file_path = ? AND guide_id = ?`, filePath, guideID)
	return err
}

// ListDrafts returns all current drafts so the guides list can flag which
// rows have unpublished edits.
func (s *Store) ListDrafts() ([]Draft, error) {
	rows, err := s.db.Query(`SELECT file_path, guide_id, payload, updated_at FROM drafts`)
	if err != nil {
		return nil, fmt.Errorf("list drafts: %w", err)
	}
	defer rows.Close()
	var out []Draft
	for rows.Next() {
		var d Draft
		var updated int64
		if err := rows.Scan(&d.FilePath, &d.GuideID, &d.Payload, &updated); err != nil {
			return nil, fmt.Errorf("scan draft: %w", err)
		}
		d.UpdatedAt = time.Unix(updated, 0)
		out = append(out, d)
	}
	return out, rows.Err()
}
