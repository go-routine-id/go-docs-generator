package cms

import "testing"

// TestCheckPassword_DifferentLengthsAllFail is the regression guard for bug
// #12: subtle.ConstantTimeCompare returns 0 fast when lengths differ, leaking
// the password length via timing. After hashing both sides to fixed-length
// SHA-256 digests the compare ALWAYS runs over 32 bytes, so length mismatches
// can no longer short-circuit. We can't directly assert timing here, but we
// can assert functional correctness: every wrong password — same length,
// shorter, longer — must return false.
func TestCheckPassword_DifferentLengthsAllFail(t *testing.T) {
	store, _ := OpenStore(":memory:")
	defer store.Close()
	auth, err := NewAuthenticator(store, "hunter2")
	if err != nil {
		t.Fatalf("NewAuthenticator: %v", err)
	}
	wrong := []string{
		"",         // empty
		"x",        // shorter
		"hunter1",  // same length, off by one
		"Hunter2",  // case differs
		"hunter22", // longer by 1
		"a very long wrong password that goes on and on", // much longer
	}
	for _, w := range wrong {
		if auth.CheckPassword(w) {
			t.Errorf("CheckPassword(%q) returned true, want false", w)
		}
	}
	if !auth.CheckPassword("hunter2") {
		t.Errorf("CheckPassword on the right password returned false")
	}
}

func TestNewAuthenticator_EmptyPasswordRejected(t *testing.T) {
	store, _ := OpenStore(":memory:")
	defer store.Close()
	if _, err := NewAuthenticator(store, ""); err == nil {
		t.Error("NewAuthenticator with empty password should fail")
	}
}
