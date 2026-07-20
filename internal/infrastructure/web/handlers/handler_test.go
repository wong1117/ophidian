package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
	"github.com/ophidian/ophidian/internal/infrastructure/web/middleware"
	"github.com/stretchr/testify/assert"
)

func setupEcho() *echo.Echo {
	e := echo.New()
	e.Use(middleware.RequestID())
	e.Use(middleware.ErrorHandler())
	return e
}

type testMissionRepo struct {
	m    *mission.Mission
	err  error
	list []*mission.Mission
}

func (r *testMissionRepo) Save(ctx context.Context, m *mission.Mission) error            { return nil }
func (r *testMissionRepo) FindByID(ctx context.Context, id string) (*mission.Mission, error) {
	return r.m, r.err
}
func (r *testMissionRepo) FindAll(ctx context.Context, filter mission.MissionFilter) ([]*mission.Mission, error) {
	return r.list, nil
}
func (r *testMissionRepo) Update(ctx context.Context, m *mission.Mission) error          { return nil }
func (r *testMissionRepo) Delete(ctx context.Context, id string) error                    { return nil }
func (r *testMissionRepo) SaveTask(ctx context.Context, task *mission.Task) error         { return nil }
func (r *testMissionRepo) FindTaskByID(ctx context.Context, id string) (*mission.Task, error) { return nil, nil }
func (r *testMissionRepo) FindTasksByMission(ctx context.Context, missionID string) ([]*mission.Task, error) {
	return nil, nil
}
func (r *testMissionRepo) UpdateTask(ctx context.Context, task *mission.Task) error       { return nil }

func TestMissionHandler_Get(t *testing.T) {
	e := setupEcho()
	m := &mission.Mission{
		ID:        common.NewID(),
		Name:      "test",
		Status:    mission.MissionActive,
		StartedBy: "op",
		Target:    mission.Target{Name: "corp"},
		CreatedAt: common.Now(),
		UpdatedAt: common.Now(),
	}
	repo := &testMissionRepo{m: m}
	h := NewMissionHandler(nil, nil, repo)

	e.GET("/api/v1/missions/:id", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/missions/"+m.ID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "test")
}

func TestMissionHandler_Get_NotFound(t *testing.T) {
	e := setupEcho()
	repo := &testMissionRepo{err: common.ErrMissionNotFound}
	h := NewMissionHandler(nil, nil, repo)

	e.GET("/api/v1/missions/:id", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/missions/bad-id", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestMissionHandler_List(t *testing.T) {
	e := setupEcho()
	repo := &testMissionRepo{
		list: []*mission.Mission{
			{ID: common.NewID(), Name: "a", Status: mission.MissionActive, CreatedAt: common.Now()},
			{ID: common.NewID(), Name: "b", Status: mission.MissionDraft, CreatedAt: common.Now()},
		},
	}
	h := NewMissionHandler(nil, nil, repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/missions", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.List(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "a")
}

func TestHealthHandler_Health(t *testing.T) {
	e := echo.New()
	h := NewHealthHandler()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Health(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
}

func TestHealthHandler_Ready(t *testing.T) {
	e := echo.New()
	h := NewHealthHandler()

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Ready(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"ready":true`)
}

func TestMissionHandler_Start(t *testing.T) {
	e := setupEcho()
	m := &mission.Mission{
		ID:        common.NewID(),
		Name:      "test",
		Status:    mission.MissionDraft,
		StartedBy: "op",
		Target:    mission.Target{Name: "corp"},
		CreatedAt: common.Now(),
		UpdatedAt: common.Now(),
	}
	repo := &testMissionRepo{m: m}
	h := NewMissionHandler(nil, nil, repo)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(m.ID.String())

	err := h.Start(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestErrorMiddleware_DomainError(t *testing.T) {
	e := setupEcho()

	e.GET("/test", func(c echo.Context) error {
		return common.ErrMissionNotFound
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "Not Found")
}

func TestErrorMiddleware_HTTPError(t *testing.T) {
	e := setupEcho()

	e.GET("/bad-request", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid input")
	})

	req := httptest.NewRequest(http.MethodGet, "/bad-request", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid input")
}

func TestErrorMiddleware_InternalError(t *testing.T) {
	e := setupEcho()

	e.GET("/panic", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusInternalServerError, "something broke")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
