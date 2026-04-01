package captive

import (
	"context"
	"fmt"
	"html"
	"net"
	"net/http"
	"time"

	"github.com/LorenzoDalBo/dns-filter/internal/identity"
)

// Credentials validates username/password.
// In the future this will check against PostgreSQL or AD/LDAP.
// Authenticator validates captive portal credentials.
// Supports both static map (for testing) and database (for production).
type Authenticator interface {
	Authenticate(username, password string) (*UserInfo, bool)
}

type UserInfo struct {
	UserID  int
	GroupID int
}

// StaticCredentials validates against an in-memory map (for testing).
type StaticCredentials struct {
	Users map[string]StaticUser
}

type StaticUser struct {
	Password string
	UserID   int
	GroupID  int
}

func (s *StaticCredentials) Authenticate(username, password string) (*UserInfo, bool) {
	user, ok := s.Users[username]
	if !ok || user.Password != password {
		return nil, false
	}
	return &UserInfo{UserID: user.UserID, GroupID: user.GroupID}, true
}

// Server serves the captive portal login page on HTTP (RF06.1).
type Server struct {
	httpServer *http.Server
	resolver   *identity.Resolver
	auth       Authenticator
	sessionTTL time.Duration
}

func NewServer(addr string, resolver *identity.Resolver, auth Authenticator, sessionTTL time.Duration) *Server {
	s := &Server{
		resolver:   resolver,
		auth:       auth,
		sessionTTL: sessionTTL,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleLogin)
	mux.HandleFunc("/auth", s.handleAuth)

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s
}

// Start begins listening for HTTP connections.
func (s *Server) Start() error {
	fmt.Printf("Captive Portal rodando em %s\n", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.httpServer.Shutdown(ctx)
}

// handleLogin serves the login page (RF06.4).
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	errMsg := html.EscapeString(r.URL.Query().Get("error"))
	redirectURL := html.EscapeString(r.URL.Query().Get("redirect"))

	errorHTML := ""
	if errMsg != "" {
		errorHTML = fmt.Sprintf(`<div class="error">%s</div>`, errMsg)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="pt-BR">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DNS Filter — Login</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
            background: #f0f2f5;
            display: flex; justify-content: center; align-items: center;
            min-height: 100vh;
        }
        .card {
            background: white; border-radius: 12px; padding: 40px;
            box-shadow: 0 2px 12px rgba(0,0,0,0.1); width: 100%%;
            max-width: 400px;
        }
        h1 { font-size: 24px; color: #1a1a2e; margin-bottom: 8px; }
        p.sub { color: #666; margin-bottom: 24px; font-size: 14px; }
        label { display: block; font-size: 14px; color: #333; margin-bottom: 4px; font-weight: 500; }
        input[type=text], input[type=password] {
            width: 100%%; padding: 10px 12px; border: 1px solid #ddd;
            border-radius: 8px; font-size: 14px; margin-bottom: 16px;
        }
        input:focus { outline: none; border-color: #4a9eed; }
        button {
            width: 100%%; padding: 12px; background: #4a9eed; color: white;
            border: none; border-radius: 8px; font-size: 16px; cursor: pointer;
            font-weight: 500;
        }
        button:hover { background: #3a8edd; }
        .error {
            background: #fee; color: #c00; padding: 10px; border-radius: 8px;
            margin-bottom: 16px; font-size: 14px; text-align: center;
        }
    </style>
</head>
<body>
    <div class="card">
        <h1>Autenticação Necessária</h1>
        <p class="sub">Faça login para acessar a internet.</p>
        %s
        <form method="POST" action="/auth">
            <input type="hidden" name="redirect" value="%s">
            <label>Usuário</label>
            <input type="text" name="username" required autofocus>
            <label>Senha</label>
            <input type="password" name="password" required>
            <button type="submit">Entrar</button>
        </form>
    </div>
</body>
</html>`, errorHTML, redirectURL)
}

// handleAuth processes the login form (RF06.3, RF06.5, RF06.6).
func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	redirectURL := r.FormValue("redirect")

	// Validate credentials via authenticator (static or database)
	user, ok := s.auth.Authenticate(username, password)
	if !ok {
		fmt.Printf("Captive: login falhou para '%s' de %s\n", username, r.RemoteAddr)
		http.Redirect(w, r, "/?error=Usuário ou senha inválidos&redirect="+redirectURL, http.StatusSeeOther)
		return
	}

	// Extract client IP from request
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "Erro interno", http.StatusInternalServerError)
		return
	}
	clientIP := net.ParseIP(host)

	// Get the group from the IP range (not from user table)
	groupID := s.resolver.GetRangeGroupID(clientIP)
	if groupID == 0 {
		groupID = user.GroupID // fallback
	}

	// Register session in Identity Resolver (RF06.3)
	s.resolver.AddSession(&identity.Session{
		ClientIP:  clientIP,
		UserID:    user.UserID,
		Username:  username,
		GroupID:   groupID,
		ExpiresAt: time.Now().Add(s.sessionTTL),
	})

	fmt.Printf("Captive: login OK — user=%s, ip=%s, group=%d\n", username, host, groupID)

	// RF06.6: redirect to original URL if available
	// Validate redirect URL to prevent open redirect (V03)
	if redirectURL != "" && isValidRedirect(redirectURL) {
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	// Default: show success page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html>
<html lang="pt-BR">
<head>
    <meta charset="UTF-8">
    <title>Login realizado</title>
    <style>
        body {
            font-family: -apple-system, sans-serif; background: #f0f2f5;
            display: flex; justify-content: center; align-items: center;
            min-height: 100vh;
        }
        .card {
            background: white; border-radius: 12px; padding: 40px;
            box-shadow: 0 2px 12px rgba(0,0,0,0.1); text-align: center;
        }
        h1 { color: #22c55e; margin-bottom: 8px; }
        p { color: #666; }
    </style>
</head>
<body>
    <div class="card">
        <h1>Login realizado com sucesso!</h1>
        <p>Você já pode navegar normalmente.</p>
    </div>
</body>
</html>`)
}

func isValidRedirect(url string) bool {
	// Allow relative paths
	if len(url) > 0 && url[0] == '/' {
		// Block protocol-relative URLs (//evil.com)
		if len(url) > 1 && url[1] == '/' {
			return false
		}
		return true
	}
	// Allow http and https only
	if len(url) > 7 && (url[:7] == "http://" || url[:8] == "https://") {
		return true
	}
	return false
}
