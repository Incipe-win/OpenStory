package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestHealthz(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewHealthHandler(nil, nil, "test-version")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/healthz", nil)

	h.Healthz(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"ok"`)
	assert.Contains(t, w.Body.String(), `"version":"test-version"`)
}

func TestReadyz_NoDeps(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewHealthHandler(nil, nil, "test-version")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/readyz", nil)

	h.Readyz(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"ok"`)
}
