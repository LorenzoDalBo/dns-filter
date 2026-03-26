package captive

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/LorenzoDalBo/dns-filter/internal/identity"
)

func setupTestServer() (*Server, *identity.Resolver) {
	resolver := identity.NewResolver(1)

	auth := &StaticCredentials{
		Users: map[string]StaticUser{
			"admin": {Password: "admin123", UserID: 1, GroupID: 2},
			"guest": {Password: "guest123", UserID: 2, GroupID: 4},
		},
	}

	server := NewServer(":0", resolver, auth, 8*time.Hour)
	return server, resolver
}

func TestLoginPageRenders(t *testing.T) {
	s, _ := setupTestServer()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	s.handleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Autenticação Necessária") {
		t.Error("Login page should contain title in PT-BR")
	}
	if !strings.Contains(body, "username") {
		t.Error("Login page should contain username field")
	}
}

func TestLoginPageShowsError(t *testing.T) {
	s, _ := setupTestServer()

	req := httptest.NewRequest("GET", "/?error=Credenciais+inválidas", nil)
	w := httptest.NewRecorder()

	s.handleLogin(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Credenciais inválidas") {
		t.Error("Login page should display error message")
	}
}

func TestAuthSuccess(t *testing.T) {
	s, resolver := setupTestServer()

	form := url.Values{}
	form.Set("username", "admin")
	form.Set("password", "admin123")

	req := httptest.NewRequest("POST", "/auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.168.1.50:12345"

	w := httptest.NewRecorder()
	s.handleAuth(w, req)

	// Should have created a session
	clientIP := net.ParseIP("192.168.1.50")
	id, _ := resolver.Resolve(clientIP)

	if id.GroupID != 2 {
		t.Errorf("Expected group 2 after login, got %d", id.GroupID)
	}
	if id.Username != "admin" {
		t.Errorf("Expected username admin, got %s", id.Username)
	}

	t.Logf("Session created: user=%s, group=%d", id.Username, id.GroupID)
}

func TestAuthFailure(t *testing.T) {
	s, resolver := setupTestServer()

	form := url.Values{}
	form.Set("username", "admin")
	form.Set("password", "wrongpassword")

	req := httptest.NewRequest("POST", "/auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.168.1.50:12345"

	w := httptest.NewRecorder()
	s.handleAuth(w, req)

	// Should redirect back with error
	if w.Code != http.StatusSeeOther {
		t.Errorf("Expected redirect 303, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if !strings.Contains(location, "error=") {
		t.Error("Failed login should redirect with error parameter")
	}

	// Should NOT have created a session
	clientIP := net.ParseIP("192.168.1.50")
	id, _ := resolver.Resolve(clientIP)
	if id.GroupID != 1 { // default group
		t.Errorf("Failed login should not create session, got group %d", id.GroupID)
	}
}

func TestAuthRedirectAfterLogin(t *testing.T) {
	s, _ := setupTestServer()

	form := url.Values{}
	form.Set("username", "guest")
	form.Set("password", "guest123")
	form.Set("redirect", "http://google.com")

	req := httptest.NewRequest("POST", "/auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "10.0.5.50:12345"

	w := httptest.NewRecorder()
	s.handleAuth(w, req)

	// Should redirect to original URL (RF06.6)
	if w.Code != http.StatusSeeOther {
		t.Errorf("Expected redirect 303, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "http://google.com" {
		t.Errorf("Expected redirect to google.com, got %s", location)
	}
}
