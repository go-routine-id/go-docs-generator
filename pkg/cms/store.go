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
	"net/url"
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
//
// Pragmas are appended to the DSN so modernc.org/sqlite runs them on EVERY
// connection it opens — running PRAGMA via db.Exec only configures the one
// pooled connection that happens to serve that call, which is a known
// foot-gun for foreign_keys and busy_timeout (both per-connection settings).
func OpenStore(path string) (*Store, error) {
	dsn := buildDSN(path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// buildDSN appends the per-connection PRAGMAs we depend on. WAL is a database-
// wide setting and persists in the file, so it only needs to run once — but
// inviting it on every connection is harmless and keeps the intent visible.
// busy_timeout makes SQLite wait up to 5s for the writer lock instead of
// returning SQLITE_BUSY immediately, which is the right tradeoff for an
// interactive admin tool where two near-simultaneous saves shouldn't 500.
// foreign_keys=ON ensures any future schema with FK references is enforced.
//
// For `:memory:` we use the `file::memory:?cache=shared` form so the SAME
// in-memory DB is shared across every pooled connection — without
// cache=shared, modernc.org/sqlite gives each pool connection a private empty
// DB, which surfaces as 'no such table' once database/sql opens a second
// connection under load. Tests pass without it only because they serialise.
//
// For file paths we use net/url to escape reserved characters (spaces, `?`,
// `#`, `%`) so paths like `/var/lib/Museum Docs/cms.db` open correctly
// instead of breaking the DSN at the first space.
func buildDSN(path string) string {
	pragmas := "_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	if path == ":memory:" {
		return "file::memory:?cache=shared&" + pragmas
	}
	u := &url.URL{Scheme: "file", Path: path, RawQuery: pragmas}
	return u.String()
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

// RekeyDraft updates a draft's file_path key, used by the startup migration
// that brings draft keys saved by the pre-EvalSymlinks binary in line with
// the resolved paths the post-fix resolver returns. If a draft already exists
// at the new key (e.g. the symlink and its target were both edited), we
// drop the old row — last writer wins, matching upsert semantics.
func (s *Store) RekeyDraft(oldPath, guideID, newPath string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`DELETE FROM drafts WHERE file_path = ? AND guide_id = ?`,
		newPath, guideID,
	); err != nil {
		return fmt.Errorf("clear target draft: %w", err)
	}
	if _, err := tx.Exec(
		`UPDATE drafts SET file_path = ? WHERE file_path = ? AND guide_id = ?`,
		newPath, oldPath, guideID,
	); err != nil {
		return fmt.Errorf("rekey draft: %w", err)
	}
	return tx.Commit()
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
