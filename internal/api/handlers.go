package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/LorenzoDalBo/dns-filter/internal/cache"
	"github.com/LorenzoDalBo/dns-filter/internal/filter"
	"github.com/LorenzoDalBo/dns-filter/internal/identity"
	"github.com/LorenzoDalBo/dns-filter/internal/logging"
	"github.com/LorenzoDalBo/dns-filter/internal/store"
)

// Handlers holds all dependencies needed by API endpoints.
// QueryMetrics provides read access to DNS handler statistics.
type QueryMetrics interface {
	QueryCount() uint64
	AvgLatencyMs() float64
}

type Handlers struct {
	store     *store.Store
	cache     *cache.Cache
	filter    *filter.Engine
	identity  *identity.Resolver
	logger    *logging.Pipeline
	black     *filter.Blacklist
	white     *filter.Blacklist
	metrics   QueryMetrics
	jwtSecret []byte
	startedAt time.Time
}

func NewHandlers(store *store.Store, cache *cache.Cache, filterEngine *filter.Engine, identityResolver *identity.Resolver, logger *logging.Pipeline, black *filter.Blacklist, white *filter.Blacklist, jwtSecret string, metrics QueryMetrics) *Handlers {
	return &Handlers{
		store:     store,
		cache:     cache,
		filter:    filterEngine,
		identity:  identityResolver,
		logger:    logger,
		black:     black,
		white:     white,
		metrics:   metrics,
		jwtSecret: []byte(jwtSecret),
		startedAt: time.Now(),
	}
}

// --- Auth ---

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// Validate against database
	user, err := h.store.AuthenticateAdmin(r.Context(), req.Username, req.Password)
	if err != nil || user == nil {
		writeError(w, "Credenciais inválidas", http.StatusUnauthorized)
		return
	}

	// Generate JWT (RF10.2)
	// Generate JWT (RF10.2, RNF03.3)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(1 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	})

	tokenString, err := token.SignedString(h.jwtSecret)
	if err != nil {
		writeError(w, "Erro ao gerar token", http.StatusInternalServerError)
		return
	}

	writeJSON(w, loginResponse{Token: tokenString})
}

// --- Middleware ---

type contextKey string

const userContextKey contextKey = "user"

type AuthUser struct {
	ID       int
	Username string
	Role     int
}

func (h *Handlers) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.Header.Get("Authorization")
		if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
			tokenStr = tokenStr[7:]
		} else {
			writeError(w, "Token não fornecido", http.StatusUnauthorized)
			return
		}

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			// Validate signing method to prevent algorithm bypass attacks (V06)
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return h.jwtSecret, nil
		})
		if err != nil || !token.Valid {
			writeError(w, "Token inválido", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			writeError(w, "Token inválido", http.StatusUnauthorized)
			return
		}

		user := AuthUser{
			ID:       int(claims["user_id"].(float64)),
			Username: claims["username"].(string),
			Role:     int(claims["role"].(float64)),
		}

		ctx := context.WithValue(r.Context(), userContextKey, &user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handlers) AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := r.Context().Value(userContextKey).(*AuthUser)
		if user.Role != 0 { // 0 = admin
			writeError(w, "Acesso negado — apenas administradores", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- Refresh Token ---

func (h *Handlers) RefreshToken(w http.ResponseWriter, r *http.Request) {
	// Get current user from context (already authenticated via middleware)
	user := r.Context().Value(userContextKey).(*AuthUser)

	// Generate new token with fresh expiration (RNF03.3)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(1 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	})

	tokenString, err := token.SignedString(h.jwtSecret)
	if err != nil {
		writeError(w, "Erro ao gerar token", http.StatusInternalServerError)
		return
	}

	writeJSON(w, loginResponse{Token: tokenString})
}

// --- Health & Metrics ---

func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

func (h *Handlers) GetMetrics(w http.ResponseWriter, r *http.Request) {
	cacheStats := h.cache.GetStats()
	uptime := time.Since(h.startedAt).Seconds()

	metrics := map[string]interface{}{
		"uptime_seconds":    int(uptime),
		"cache_hits":        cacheStats.Hits,
		"cache_misses":      cacheStats.Misses,
		"cache_entries":     h.cache.Size(),
		"blacklist_domains": h.black.Size(),
		"whitelist_domains": h.white.Size(),
		"active_sessions":   h.identity.SessionCount(),
	}

	if h.metrics != nil {
		totalQueries := h.metrics.QueryCount()
		metrics["total_queries"] = totalQueries
		metrics["avg_latency_ms"] = h.metrics.AvgLatencyMs()
		if uptime > 0 {
			metrics["queries_per_second"] = float64(totalQueries) / uptime
		}
	}

	if h.logger != nil {
		metrics["log_pending"] = h.logger.Pending()
	}

	writeJSON(w, metrics)
}

// --- Cache ---

func (h *Handlers) InvalidateCache(w http.ResponseWriter, r *http.Request) {
	domain := chi.URLParam(r, "domain")
	removed := h.cache.Invalidate(domain)
	writeJSON(w, map[string]interface{}{
		"domain":  domain,
		"removed": removed,
	})
}

// --- Logs ---

func (h *Handlers) GetLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	offset, _ := strconv.Atoi(q.Get("offset"))

	logs, total, err := h.store.QueryLogs(r.Context(), store.LogFilter{
		ClientIP: q.Get("client_ip"),
		Domain:   q.Get("domain"),
		Action:   q.Get("action"),
		DateFrom: q.Get("date_from"),
		DateTo:   q.Get("date_to"),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		writeError(w, fmt.Sprintf("Erro ao consultar logs: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"data":   logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// --- Users ---

func (h *Handlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListAdminUsers(r.Context())
	if err != nil {
		writeError(w, fmt.Sprintf("Erro: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, users)
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     int    `json:"role"`
}

func (h *Handlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	id, err := h.store.CreateAdminUser(r.Context(), req.Username, req.Password, req.Role)
	if err != nil {
		writeError(w, fmt.Sprintf("Erro ao criar usuário: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]int{"id": id})
}

// --- Groups ---

func (h *Handlers) ListGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.store.ListGroups(r.Context())
	if err != nil {
		writeError(w, fmt.Sprintf("Erro: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, groups)
}

type createGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *Handlers) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var req createGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	id, err := h.store.CreateGroup(r.Context(), req.Name, req.Description)
	if err != nil {
		writeError(w, fmt.Sprintf("Erro ao criar grupo: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]int{"id": id})
}

// --- Blocklists ---

func (h *Handlers) ListBlocklists(w http.ResponseWriter, r *http.Request) {
	lists, err := h.store.ListBlocklists(r.Context())
	if err != nil {
		writeError(w, fmt.Sprintf("Erro: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, lists)
}

type createBlocklistRequest struct {
	Name      string `json:"name"`
	SourceURL string `json:"source_url"`
	ListType  int    `json:"list_type"`
}

func (h *Handlers) CreateBlocklist(w http.ResponseWriter, r *http.Request) {
	var req createBlocklistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	id, err := h.store.InsertBlocklist(r.Context(), req.Name, req.SourceURL, req.ListType)
	if err != nil {
		writeError(w, fmt.Sprintf("Erro: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]int{"id": id})
}

type addEntriesRequest struct {
	Domains []string `json:"domains"`
}

func (h *Handlers) AddEntries(w http.ResponseWriter, r *http.Request) {
	listID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "ID inválido", http.StatusBadRequest)
		return
	}

	var req addEntriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	count, err := h.store.InsertBlocklistEntries(r.Context(), listID, req.Domains)
	if err != nil {
		writeError(w, fmt.Sprintf("Erro: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]int{"inserted": count})
}

func (h *Handlers) ReloadLists(w http.ResponseWriter, r *http.Request) {
	blackDomains, whiteDomains, err := h.store.LoadActiveBlocklistEntries(r.Context())
	if err != nil {
		writeError(w, fmt.Sprintf("Erro ao recarregar: %v", err), http.StatusInternalServerError)
		return
	}

	// Rebuild in-memory lists
	newBlack := filter.NewBlacklist()
	newWhite := filter.NewBlacklist()
	for _, d := range blackDomains {
		newBlack.Add(d)
	}
	for _, d := range whiteDomains {
		newWhite.Add(d)
	}

	// Swap — for now we update the existing lists
	// In a future refactor, Engine could support atomic swap
	h.black.Clear()
	h.white.Clear()
	for _, d := range blackDomains {
		h.black.Add(d)
	}
	for _, d := range whiteDomains {
		h.white.Add(d)
	}

	writeJSON(w, map[string]interface{}{
		"blacklist": len(blackDomains),
		"whitelist": len(whiteDomains),
		"status":    "reloaded",
	})
}

// --- IP Ranges ---

func (h *Handlers) ListRanges(w http.ResponseWriter, r *http.Request) {
	ranges, err := h.store.ListIPRanges(r.Context())
	if err != nil {
		writeError(w, fmt.Sprintf("Erro: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, ranges)
}

type createRangeRequest struct {
	CIDR        string `json:"cidr"`
	GroupID     int    `json:"group_id"`
	AuthMode    int    `json:"auth_mode"`
	Description string `json:"description"`
}

func (h *Handlers) CreateRange(w http.ResponseWriter, r *http.Request) {
	var req createRangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	_, cidrNet, err := net.ParseCIDR(req.CIDR)
	if err != nil {
		writeError(w, "CIDR inválido", http.StatusBadRequest)
		return
	}

	id, err := h.store.CreateIPRange(r.Context(), req.CIDR, req.GroupID, req.AuthMode, req.Description)
	if err != nil {
		writeError(w, fmt.Sprintf("Erro: %v", err), http.StatusInternalServerError)
		return
	}

	// Update in-memory identity resolver
	// Append to in-memory identity resolver (don't replace existing ranges)
	h.identity.AddRange(identity.IPRange{
		Network:  cidrNet,
		GroupID:  req.GroupID,
		AuthMode: identity.AuthMode(req.AuthMode),
	})

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]int{"id": id})
}

// --- Update/Delete Users ---

type updateUserRequest struct {
	Username string `json:"username"`
	Role     int    `json:"role"`
	Active   bool   `json:"active"`
}

func (h *Handlers) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "ID inválido", http.StatusBadRequest)
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateAdminUser(r.Context(), id, req.Username, req.Role, req.Active); err != nil {
		writeError(w, fmt.Sprintf("Erro: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "updated"})
}

func (h *Handlers) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "ID inválido", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteAdminUser(r.Context(), id); err != nil {
		writeError(w, fmt.Sprintf("Erro: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "deleted"})
}

// --- Update/Delete Groups ---

func (h *Handlers) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "ID inválido", http.StatusBadRequest)
		return
	}

	var req createGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateGroup(r.Context(), id, req.Name, req.Description); err != nil {
		writeError(w, fmt.Sprintf("Erro: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "updated"})
}

func (h *Handlers) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "ID inválido", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteGroup(r.Context(), id); err != nil {
		writeError(w, fmt.Sprintf("Erro: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "deleted"})
}

// --- Update/Delete Blocklists ---

type updateBlocklistRequest struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

func (h *Handlers) UpdateBlocklist(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "ID inválido", http.StatusBadRequest)
		return
	}

	var req updateBlocklistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateBlocklist(r.Context(), id, req.Name, req.Active); err != nil {
		writeError(w, fmt.Sprintf("Erro: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "updated"})
}

func (h *Handlers) DeleteBlocklist(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "ID inválido", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteBlocklist(r.Context(), id); err != nil {
		writeError(w, fmt.Sprintf("Erro: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "deleted"})
}

// --- Update/Delete Ranges ---

func (h *Handlers) UpdateRange(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "ID inválido", http.StatusBadRequest)
		return
	}

	var req createRangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateIPRange(r.Context(), id, req.CIDR, req.GroupID, req.AuthMode, req.Description); err != nil {
		writeError(w, fmt.Sprintf("Erro: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "updated"})
}

func (h *Handlers) DeleteRange(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, "ID inválido", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteIPRange(r.Context(), id); err != nil {
		writeError(w, fmt.Sprintf("Erro: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "deleted"})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
