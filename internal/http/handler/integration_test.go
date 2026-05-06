package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Incipe-win/OpenStory/internal/audit"
	"github.com/Incipe-win/OpenStory/internal/auth"
	"github.com/Incipe-win/OpenStory/internal/config"
	"github.com/Incipe-win/OpenStory/internal/db"
	"github.com/Incipe-win/OpenStory/internal/http/handler"
	"github.com/Incipe-win/OpenStory/internal/http/middleware"
	"github.com/Incipe-win/OpenStory/internal/http/router"
)

// testEnv provides a test environment with a real database.
// Requires: docker compose up -d postgres redis
type testEnv struct {
	engine *gin.Engine
	pool   *pgxpool.Pool
	t      *testing.T
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	cfg, err := config.Load()
	require.NoError(t, err, "load config — is DATABASE_URL set or infra running?")

	pool, err := db.NewPool(context.Background(), cfg.Database.URL)
	if err != nil {
		t.Skipf("skipping integration test: database not available: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	engine := router.New(router.Deps{
		Log:    zerolog.Nop(),
		Pool:   pool,
		RDB:    nil, // Redis optional for tests
		Config: cfg,
	})

	env := &testEnv{engine: engine, pool: pool, t: t}
	return env
}

// cleanupAuthTestData removes test-auth user and associated data.
func (e *testEnv) cleanupAuthTestData() {
	ctx := context.Background()
	_, err := e.pool.Exec(ctx, "DELETE FROM audit_logs WHERE ip_address = ''")
	require.NoError(e.t, err)
	_, err = e.pool.Exec(ctx, "DELETE FROM refresh_tokens WHERE user_id IN (SELECT id FROM users WHERE email = 'test-auth@example.com')")
	require.NoError(e.t, err)
	_, err = e.pool.Exec(ctx, "DELETE FROM users WHERE email = 'test-auth@example.com'")
	require.NoError(e.t, err)
}

func (e *testEnv) doJSON(method, path string, body any, token string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		require.NoError(e.t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

func (e *testEnv) parseBody(w *httptest.ResponseRecorder) map[string]any {
	var result map[string]any
	require.NoError(e.t, json.Unmarshal(w.Body.Bytes(), &result))
	return result
}

// ─── Auth Tests ─────────────────────────────────────

func TestAuthFlow(t *testing.T) {
	env := setupTestEnv(t)

	// Clean up before and after test
	env.cleanupAuthTestData()
	t.Cleanup(func() { env.cleanupAuthTestData() })

	// 1. Register
	w := env.doJSON("POST", "/api/auth/register", map[string]string{
		"email": "test-auth@example.com", "username": "testauthuser",
		"password": "testpassword123", "display_name": "Test User",
	}, "")
	assert.Equal(t, http.StatusCreated, w.Code, "register should return 201: %s", w.Body.String())
	body := env.parseBody(w)
	data := body["data"].(map[string]any)
	assert.Equal(t, "test-auth@example.com", data["email"])
	assert.Equal(t, "user", data["role"])

	// 2. Register duplicate email
	w = env.doJSON("POST", "/api/auth/register", map[string]string{
		"email": "test-auth@example.com", "username": "testauthuser2",
		"password": "testpassword123",
	}, "")
	assert.Equal(t, http.StatusConflict, w.Code, "duplicate email should 409")

	// 3. Login
	w = env.doJSON("POST", "/api/auth/login", map[string]string{
		"email": "test-auth@example.com", "password": "testpassword123",
	}, "")
	assert.Equal(t, http.StatusOK, w.Code, "login should return 200: %s", w.Body.String())
	body = env.parseBody(w)
	data = body["data"].(map[string]any)
	accessToken := data["access_token"].(string)
	refreshToken := data["refresh_token"].(string)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)

	// 4. Login bad password
	w = env.doJSON("POST", "/api/auth/login", map[string]string{
		"email": "test-auth@example.com", "password": "wrongpassword",
	}, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// 5. Get /api/me
	w = env.doJSON("GET", "/api/me", nil, accessToken)
	assert.Equal(t, http.StatusOK, w.Code, "me should return 200: %s", w.Body.String())
	body = env.parseBody(w)
	data = body["data"].(map[string]any)
	assert.Equal(t, "test-auth@example.com", data["email"])

	// 6. Get /api/me without token
	w = env.doJSON("GET", "/api/me", nil, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// 7. Refresh token
	w = env.doJSON("POST", "/api/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	}, "")
	assert.Equal(t, http.StatusOK, w.Code, "refresh should return 200: %s", w.Body.String())
	body = env.parseBody(w)
	data = body["data"].(map[string]any)
	newAccessToken := data["access_token"].(string)
	newRefreshToken := data["refresh_token"].(string)
	assert.NotEmpty(t, newAccessToken)
	assert.NotEmpty(t, newRefreshToken)
	assert.NotEqual(t, refreshToken, newRefreshToken, "new refresh token should differ")

	// 8. Old refresh token should be revoked
	w = env.doJSON("POST", "/api/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	}, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code, "old refresh token should be rejected")
}

// ─── Project Tests ──────────────────────────────────

func TestProjectCRUD(t *testing.T) {
	env := setupTestEnv(t)

	// Get JWT for the seeded demo user
	cfg, _ := config.Load()
	jwtSvc := auth.NewJWTService(cfg.JWT.Secret, cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL)

	ctx := context.Background()
	authRepo := auth.NewPgRepository(env.pool)
	demoUser, err := authRepo.GetUserByEmail(ctx, "demo@openstory.local")
	require.NoError(t, err)

	tokenPair, _, _ := jwtSvc.GenerateTokenPair(demoUser.ID, demoUser.Username, demoUser.Role)
	token := tokenPair.AccessToken

	// Cleanup before and after
	cleanupProjects := func() {
		_, err := env.pool.Exec(context.Background(), "DELETE FROM audit_logs WHERE ip_address = ''")
		require.NoError(t, err)
		_, err = env.pool.Exec(context.Background(), "DELETE FROM projects WHERE user_id = $1 AND name LIKE 'Test Project%'", demoUser.ID)
		require.NoError(t, err)
	}
	cleanupProjects()
	t.Cleanup(cleanupProjects)

	// 1. Create project
	w := env.doJSON("POST", "/api/projects", map[string]string{
		"name": "Test Project Alpha", "description": "A test project",
	}, token)
	assert.Equal(t, http.StatusCreated, w.Code, "create should 201: %s", w.Body.String())
	body := env.parseBody(w)
	data := body["data"].(map[string]any)
	projectID := data["id"].(string)
	assert.Equal(t, "Test Project Alpha", data["name"])
	assert.Equal(t, "draft", data["status"])

	// 2. Create without name (validation)
	w = env.doJSON("POST", "/api/projects", map[string]string{}, token)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 3. List projects
	w = env.doJSON("GET", "/api/projects?page=1&page_size=10", nil, token)
	assert.Equal(t, http.StatusOK, w.Code)
	body = env.parseBody(w)
	meta := body["meta"].(map[string]any)
	assert.True(t, meta["total"].(float64) >= 1)

	// 4. Get project
	w = env.doJSON("GET", "/api/projects/"+projectID, nil, token)
	assert.Equal(t, http.StatusOK, w.Code)
	body = env.parseBody(w)
	data = body["data"].(map[string]any)
	assert.Equal(t, "Test Project Alpha", data["name"])

	// 5. Update project
	w = env.doJSON("PATCH", "/api/projects/"+projectID, map[string]string{
		"name": "Test Project Beta",
	}, token)
	assert.Equal(t, http.StatusOK, w.Code)
	body = env.parseBody(w)
	data = body["data"].(map[string]any)
	assert.Equal(t, "Test Project Beta", data["name"])

	// 6. Get non-existent project
	w = env.doJSON("GET", "/api/projects/00000000-0000-0000-0000-000000099999", nil, token)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// 7. Feed (public, no auth)
	w = env.doJSON("GET", "/api/feed", nil, "")
	assert.Equal(t, http.StatusOK, w.Code)
	body = env.parseBody(w)
	assert.NotNil(t, body["data"])
	assert.NotNil(t, body["meta"])
}

// ─── Middleware Tests ───────────────────────────────

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg, _ := config.Load()
	jwtSvc := auth.NewJWTService(cfg.JWT.Secret, cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL)

	r := gin.New()
	r.Use(middleware.Auth(jwtSvc))
	r.GET("/test", func(c *gin.Context) {
		uid := middleware.GetUserID(c)
		c.JSON(200, gin.H{"user_id": uid.String()})
	})

	// No header
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Bad header format
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic abc")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Invalid token
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ─── Response Tests ─────────────────────────────────

func TestUnifiedErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	auditLog := &audit.Logger{} // nil pool, won't be used
	projH := handler.NewProjectHandler(nil, auditLog, nil, zerolog.Nop(), nil)

	r.GET("/projects/:id", projH.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/projects/not-a-uuid", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	errBody := body["error"].(map[string]any)
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])
}
