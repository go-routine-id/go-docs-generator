package cms

import (
	"testing"
	"time"
)

// openTestStore returns a fresh in-memory SQLite store. Each test gets its own
// DB so cases stay independent.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(":memory:")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestStore_SessionRoundTrip(t *testing.T) {
	s := openTestStore(t)

	sess, err := s.NewSession(AdminUser)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if sess.Token == "" {
		t.Fatal("empty session token — should never happen")
	}
	if !sess.ExpiresAt.After(time.Now()) {
		t.Errorf("session expires_at must be in the future, got %v", sess.ExpiresAt)
	}

	got, err := s.LookupSession(sess.Token)
	if err != nil {
		t.Fatalf("LookupSession: %v", err)
	}
	if got.User != AdminUser {
		t.Errorf("LookupSession user = %q, want %q", got.User, AdminUser)
	}
}

func TestStore_LookupUnknownTokenIsNotFound(t *testing.T) {
	s := openTestStore(t)
	_, err := s.LookupSession("definitely-not-a-real-token")
	if err != ErrSessionNotFound {
		t.Errorf("err = %v, want ErrSessionNotFound", err)
	}
}

func TestStore_DeleteSession(t *testing.T) {
	s := openTestStore(t)
	sess, err := s.NewSession(AdminUser)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if err := s.DeleteSession(sess.Token); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := s.LookupSession(sess.Token); err != ErrSessionNotFound {
		t.Errorf("after delete: err = %v, want ErrSessionNotFound", err)
	}
}

// TestStore_GarbageCollect manually pokes an already-expired row in and
// verifies the GC sweep removes it. We bypass NewSession because that path
// always inserts a future expires_at — GC behaviour is what we're after.
func TestStore_GarbageCollect(t *testing.T) {
	s := openTestStore(t)
	past := time.Now().Add(-1 * time.Hour).Unix()
	if _, err := s.db.Exec(
		`INSERT INTO sessions (token, user, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		"stale-tok", AdminUser, past-3600, past,
	); err != nil {
		t.Fatalf("seed expired row: %v", err)
	}

	if err := s.GarbageCollect(); err != nil {
		t.Fatalf("GarbageCollect: %v", err)
	}
	if _, err := s.LookupSession("stale-tok"); err != ErrSessionNotFound {
		t.Errorf("expired row should be gone, got err = %v", err)
	}
}

func TestStore_DeleteUnknownTokenIsNoOp(t *testing.T) {
	s := openTestStore(t)
	if err := s.DeleteSession("nope"); err != nil {
		t.Errorf("deleting unknown token should be no-op, got: %v", err)
	}
}
