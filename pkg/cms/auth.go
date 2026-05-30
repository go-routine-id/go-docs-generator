package cms

import (
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AdminUser is the identity attached to a logged-in admin session. Single-user
// for the MVP; structured this way so multi-user later is a contained change.
const AdminUser = "admin"

// sessionCookieName is the cookie key for the opaque session token. Stable
// across versions — changing it would log every admin out on next deploy.
const sessionCookieName = "cms_session"

// ContextSessionKey is the gin context key for the validated Session pointer.
// Handlers behind RequireAuth can read this to know who's calling.
const ContextSessionKey = "cms.session"

// Authenticator validates submitted passwords and mints session cookies.
type Authenticator struct {
	store    *Store
	password string // plaintext expected; compared in constant time
}

// NewAuthenticator returns an Authenticator. password must be non-empty —
// callers (main) should fail fast on startup if env CMS_ADMIN_PASSWORD is unset.
func NewAuthenticator(store *Store, password string) (*Authenticator, error) {
	if password == "" {
		return nil, errors.New("admin password is empty — set CMS_ADMIN_PASSWORD")
	}
	return &Authenticator{store: store, password: password}, nil
}

// CheckPassword returns true iff submitted matches the configured password.
// Constant-time compare avoids leaking length/prefix via timing.
func (a *Authenticator) CheckPassword(submitted string) bool {
	return subtle.ConstantTimeCompare([]byte(submitted), []byte(a.password)) == 1
}

// Login mints a fresh session, writes the cookie, and returns the session.
// Caller is responsible for redirecting/returning OK after this.
func (a *Authenticator) Login(c *gin.Context) (*Session, error) {
	sess, err := a.store.NewSession(AdminUser)
	if err != nil {
		return nil, err
	}
	a.setSessionCookie(c, sess.Token, int(SessionTTL.Seconds()))
	return sess, nil
}

// Logout deletes the session row and clears the cookie.
func (a *Authenticator) Logout(c *gin.Context) error {
	if tok, err := c.Cookie(sessionCookieName); err == nil && tok != "" {
		_ = a.store.DeleteSession(tok)
	}
	a.setSessionCookie(c, "", -1)
	return nil
}

// setSessionCookie writes the session cookie with SameSite=Strict + HttpOnly,
// which is enough to prevent CSRF on POST mutations from a foreign origin.
// Secure is set when the request came in over HTTPS — local HTTP dev still works.
func (a *Authenticator) setSessionCookie(c *gin.Context, token string, maxAge int) {
	secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(sessionCookieName, token, maxAge, "/", "", secure, true)
}

// RequireAuth is gin middleware that aborts with 302 → /login (for HTML
// requests) or 401 (for everything else) when the request has no valid
// session. On success it stores the *Session in the context under
// ContextSessionKey so downstream handlers can introspect it.
func (a *Authenticator) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tok, err := c.Cookie(sessionCookieName)
		if err != nil || tok == "" {
			a.rejectUnauthorized(c)
			return
		}
		sess, err := a.store.LookupSession(tok)
		if err != nil {
			a.rejectUnauthorized(c)
			return
		}
		c.Set(ContextSessionKey, sess)
		c.Next()
	}
}

func (a *Authenticator) rejectUnauthorized(c *gin.Context) {
	if acceptsHTML(c) {
		c.Redirect(http.StatusFound, "/login")
	} else {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	c.Abort()
}

// acceptsHTML returns true when the client's Accept header includes text/html.
// Browsers always do; fetch()/curl typically don't.
func acceptsHTML(c *gin.Context) bool {
	for _, v := range c.Request.Header.Values("Accept") {
		if containsCI(v, "text/html") {
			return true
		}
	}
	return false
}

// containsCI is a tiny case-insensitive substring check so we don't pull in
// strings.EqualFold ceremony for the one place this matters.
func containsCI(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	if len(haystack) < len(needle) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			a, b := haystack[i+j], needle[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
