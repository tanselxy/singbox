package panel

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookie = "sb_session"
	sessionTTL    = 12 * time.Hour

	lockoutThreshold = 5                // failures before lockout
	lockoutWindow    = 10 * time.Minute // window failures are counted in
	lockoutDuration  = 15 * time.Minute // how long a locked ip stays locked
)

// auth handles session issuance/validation and brute-force lockout.
type auth struct {
	key []byte

	mu       sync.RWMutex
	attempts map[string]*attemptState
}

type attemptState struct {
	count      int
	windowFrom time.Time
	lockedTil  time.Time
}

func newAuth(hexKey string) (*auth, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("decode session key: %w", err)
	}
	return &auth{key: key, attempts: map[string]*attemptState{}}, nil
}

// issueSession sets a signed session cookie valid for sessionTTL.
func (a *auth) issueSession(w http.ResponseWriter, secure bool) {
	exp := time.Now().Add(sessionTTL).Unix()
	value := fmt.Sprintf("%d.%s", exp, a.sign(strconv.FormatInt(exp, 10)))
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(exp, 0),
	})
}

// clearSession removes the session cookie.
func (a *auth) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// validSession reports whether the request carries a valid, unexpired session.
func (a *auth) validSession(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	expStr, sig, ok := strings.Cut(c.Value, ".")
	if !ok {
		return false
	}
	if !hmac.Equal([]byte(sig), []byte(a.sign(expStr))) {
		return false
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return false
	}
	return time.Now().Unix() < exp
}

func (a *auth) sign(msg string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	m := hmac.New(sha256.New, a.key)
	m.Write([]byte(msg))
	return hex.EncodeToString(m.Sum(nil))
}

// rotateKey invalidates all current sessions by replacing their signing key.
func (a *auth) rotateKey(hexKey string) error {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return fmt.Errorf("decode session key: %w", err)
	}
	a.mu.Lock()
	a.key = key
	a.attempts = map[string]*attemptState{}
	a.mu.Unlock()
	return nil
}

// locked reports whether ip is currently locked out.
func (a *auth) locked(ip string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.attempts[ip]
	return st != nil && time.Now().Before(st.lockedTil)
}

// recordFailure counts a failed login and may trigger a lockout.
func (a *auth) recordFailure(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	st := a.attempts[ip]
	if st == nil || now.Sub(st.windowFrom) > lockoutWindow {
		st = &attemptState{windowFrom: now}
		a.attempts[ip] = st
	}
	st.count++
	if st.count >= lockoutThreshold {
		st.lockedTil = now.Add(lockoutDuration)
		st.count = 0
		st.windowFrom = now
	}
}

// recordSuccess clears any failure state for ip.
func (a *auth) recordSuccess(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.attempts, ip)
}
