package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"deploy-platform/internal/platform/httpx"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

type App struct {
	config Config
	db     *sql.DB
	logger *slog.Logger
}

func New(config Config, db *sql.DB) *App {
	return &App{config: config, db: db, logger: slog.New(slog.NewJSONHandler(os.Stdout, nil))}
}

func (app *App) Routes() http.Handler {
	router := chi.NewRouter()
	router.Use(app.requestLogger)
	router.Get("/api/health", app.health)
	router.Post("/api/auth/login", app.login)
	router.Get("/api/deployments/{id}/approve", app.approveDeploymentByToken)
	router.Get("/api/deployments/{id}/reject", app.rejectDeploymentByToken)
	router.Handle("GET /api/roles", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.listRoles))))
	router.Handle("POST /api/roles", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.createRole))))
	router.Handle("PUT /api/roles/{id}", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.updateRole))))
	router.Handle("DELETE /api/roles/{id}", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.deleteRole))))
	router.Handle("GET /api/permissions", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.listPermissions))))
	router.Handle("POST /api/permissions", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.createPermission))))
	router.Handle("PUT /api/permissions/{id}", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.updatePermission))))
	router.Handle("DELETE /api/permissions/{id}", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.deletePermission))))
	router.Handle("GET /api/users", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.listUsers))))
	router.Handle("POST /api/users", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.createUser))))
	router.Handle("PUT /api/users/{id}", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.updateUser))))
	router.Handle("DELETE /api/users/{id}", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.deleteUser))))
	router.Post("/api/callback/deploy", app.deployCallback)
	router.Handle("GET /api/projects", app.requireAuth(http.HandlerFunc(app.projects)))
	router.Handle("GET /api/projects/{id}", app.requireAuth(http.HandlerFunc(app.projectDetail)))
	router.Handle("PUT /api/projects/{id}", app.requireAuth(http.HandlerFunc(app.updateProject)))
	router.Handle("DELETE /api/projects/{id}", app.requireAuth(http.HandlerFunc(app.deleteProject)))
	router.Handle("GET /api/projects/{project_id}/deployments", app.requireAuth(http.HandlerFunc(app.deployments)))
	router.Handle("GET /api/deployments", app.requireAuth(http.HandlerFunc(app.allDeployments)))
	router.Handle("POST /api/projects", app.requireAuth(http.HandlerFunc(app.createProject)))
	router.Handle("GET /api/teams", app.requireAuth(http.HandlerFunc(app.teams)))
	router.Handle("POST /api/teams", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.createTeam))))
	router.Handle("PUT /api/teams/{id}", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.updateTeam))))
	router.Handle("DELETE /api/teams/{id}", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.deleteTeam))))
	router.Handle("GET /api/nodes", app.requireAuth(http.HandlerFunc(app.nodes)))
	router.Handle("POST /api/nodes", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.createNode))))
	router.Handle("PUT /api/nodes/{id}", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.updateNode))))
	router.Handle("DELETE /api/nodes/{id}", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.deleteNode))))
	router.Handle("POST /api/deploy", app.requireAuth(http.HandlerFunc(app.deploy)))
	router.Handle("POST /api/deployments/{id}/approve", app.requireAuth(http.HandlerFunc(app.approveDeployment)))
	router.Handle("POST /api/deployments/{id}/reject", app.requireAuth(http.HandlerFunc(app.rejectDeployment)))
	router.Handle("POST /api/deployments/{id}/rollback", app.requireAuth(http.HandlerFunc(app.rollbackDeployment)))
	router.Handle("POST /api/deployments/{id}/retry", app.requireAuth(http.HandlerFunc(app.retryDeployment)))
	router.Handle("GET /api/deployments/{id}", app.requireAuth(http.HandlerFunc(app.deploymentDetail)))
	router.Handle("GET /api/notification-channels", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.listNotificationChannels))))
	router.Handle("POST /api/notification-channels", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.createNotificationChannel))))
	router.Handle("PUT /api/notification-channels/{id}", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.updateNotificationChannel))))
	router.Handle("DELETE /api/notification-channels/{id}", app.requireAuth(app.requireAdmin(http.HandlerFunc(app.deleteNotificationChannel))))
	app.serveSPA(router)
	return router
}

func (app *App) health(w http.ResponseWriter, _ *http.Request) {
	if err := app.db.Ping(); err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (app *App) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := httpx.Decode(w, r, 4<<10, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var id, tokenVersion int64
	var passwordHash string
	var disabled, isAdmin bool
	err := app.db.QueryRowContext(
		r.Context(),
		`SELECT id, password_hash, token_version, disabled, CAST(is_admin AS INTEGER)
		 FROM users WHERE username = ?`,
		request.Username,
	).Scan(&id, &passwordHash, &tokenVersion, &disabled, &isAdmin)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err != nil {
		app.serverError(w, err)
		return
	}
	if disabled || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(request.Password)) != nil {
		httpx.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := SignAccessToken(app.config.JWTSecret, id, tokenVersion, time.Now())
	if err != nil {
		app.serverError(w, err)
		return
	}
	roles, permissions, err := app.userRolesAndPermissions(r.Context(), id, isAdmin)
	if err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"access_token": token,
		"roles":        roles,
		"permissions":  permissions,
	})
}

type projectView struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	TeamID         int64  `json:"team_id"`
	CurrentVersion string `json:"current_version"`
	LatestDeployed string `json:"latest_deployed_at"`
	CreatedAt      string `json:"created_at"`
}

type updateProjectRequest struct {
	Name      string `json:"name"`
	TeamID    int64  `json:"team_id"`
	NodeID    int64  `json:"node_id"`
	ImageRepo string `json:"image_repo"`
}

func (app *App) projects(w http.ResponseWriter, r *http.Request) {
	operator := currentUser(r)
	query := `
		SELECT p.id, p.name, p.team_id, p.current_version,
		       COALESCE((SELECT MAX(d.created_at) FROM deploy_snapshots d WHERE d.project_id = p.id), ''),
		       p.created_at
		FROM projects p
		`
	args := make([]any, 0)
	if !operator.IsAdmin {
		query += `JOIN user_projects up ON up.project_id = p.id AND up.user_id = ? `
		args = append(args, operator.ID)
	}
	query += `ORDER BY p.name`
	rows, err := app.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer rows.Close()

	projects := make([]projectView, 0)
	for rows.Next() {
		var project projectView
		if err := rows.Scan(&project.ID, &project.Name, &project.TeamID, &project.CurrentVersion, &project.LatestDeployed, &project.CreatedAt); err != nil {
			app.serverError(w, err)
			return
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, projects)
}

func (app *App) projectDetail(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}
	project, ok := app.canAccessProject(w, r, id, "project:view")
	if !ok {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":              project.ID,
		"name":            project.Name,
		"team_id":         project.TeamID,
		"node_id":         project.NodeID,
		"current_yaml":    project.CurrentYAML,
		"current_version": project.CurrentVersion,
		"image_repo":      project.ImageRepo,
	})
}

type createProjectRequest struct {
	Name        string `json:"name"`
	TeamID      int64  `json:"team_id"`
	NodeID      int64  `json:"node_id"`
	CurrentYAML string `json:"current_yaml"`
	CurrentVer  string `json:"current_version"`
	ImageRepo   string `json:"image_repo"`
}

func (app *App) createProject(w http.ResponseWriter, r *http.Request) {
	var request createProjectRequest
	if err := httpx.Decode(w, r, 1<<20, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	request.Name = strings.ToLower(request.Name)
	if !projectNamePattern.MatchString(request.Name) {
		httpx.Error(w, http.StatusBadRequest, "invalid project name")
		return
	}
	if request.CurrentYAML == "" {
		httpx.Error(w, http.StatusBadRequest, "current_yaml is required")
		return
	}
	if !app.canAccessTeam(w, r, request.TeamID, "project:create") {
		return
	}

	result, err := app.db.ExecContext(r.Context(), `
		INSERT INTO projects(name, team_id, node_id, current_yaml, current_version, image_repo)
		VALUES (?, ?, ?, ?, ?, ?)`,
		request.Name, request.TeamID, request.NodeID, request.CurrentYAML, request.CurrentVer, request.ImageRepo)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, http.StatusConflict, "project already exists")
			return
		}
		app.serverError(w, err)
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, projectView{ID: id, Name: request.Name, TeamID: request.TeamID, CurrentVersion: request.CurrentVer})
}

func (app *App) updateProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}
	var request updateProjectRequest
	if err := httpx.Decode(w, r, 1<<20, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	request.Name = strings.ToLower(request.Name)
	if !projectNamePattern.MatchString(request.Name) {
		httpx.Error(w, http.StatusBadRequest, "invalid project name")
		return
	}
	if _, ok := app.canAccessProject(w, r, id, "project:update"); !ok {
		return
	}
	if err := app.ensureProjectReferences(r.Context(), request.TeamID, request.NodeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.Error(w, http.StatusBadRequest, "team or node not found")
			return
		}
		app.serverError(w, err)
		return
	}
	if !app.canAccessTeam(w, r, request.TeamID, "project:update") {
		return
	}
	if _, err := app.db.ExecContext(r.Context(), `
		UPDATE projects
		SET name = ?, team_id = ?, node_id = ?, image_repo = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		request.Name, request.TeamID, request.NodeID, request.ImageRepo, id,
	); err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, http.StatusConflict, "project already exists")
			return
		}
		app.serverError(w, err)
		return
	}
	project, err := app.getProject(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":              project.ID,
		"name":            project.Name,
		"team_id":         project.TeamID,
		"node_id":         project.NodeID,
		"current_yaml":    project.CurrentYAML,
		"current_version": project.CurrentVersion,
	})
}

func (app *App) deleteProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if _, ok := app.canAccessProject(w, r, id, "project:delete"); !ok {
		return
	}

	var running int
	if err := app.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM deploy_snapshots WHERE project_id = ? AND status IN ('running', 'pending')`, id).Scan(&running); err != nil {
		app.serverError(w, err)
		return
	}
	if running > 0 {
		httpx.Error(w, http.StatusConflict, "project has a running deployment")
		return
	}
	if _, err := app.db.ExecContext(r.Context(), `DELETE FROM deploy_snapshots WHERE project_id = ?`, id); err != nil {
		app.serverError(w, err)
		return
	}
	if _, err := app.db.ExecContext(r.Context(), `DELETE FROM projects WHERE id = ?`, id); err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (app *App) ensureProjectReferences(ctx context.Context, teamID, nodeID int64) error {
	var teamExists, nodeExists int
	if err := app.db.QueryRowContext(ctx, `SELECT 1 FROM teams WHERE id = ?`, teamID).Scan(&teamExists); err != nil {
		return err
	}
	if err := app.db.QueryRowContext(ctx, `SELECT 1 FROM nodes WHERE id = ?`, nodeID).Scan(&nodeExists); err != nil {
		return err
	}
	return nil
}

func (app *App) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := ParseAccessToken(app.config.JWTSecret, bearerToken(r))
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var tokenVersion int64
		var username string
		var isAdmin, disabled int
		err = app.db.QueryRowContext(
			r.Context(),
			`SELECT token_version, username, CAST(is_admin AS INTEGER), CAST(disabled AS INTEGER) FROM users WHERE id = ?`,
			claims.UserID,
		).Scan(&tokenVersion, &username, &isAdmin, &disabled)
		if errors.Is(err, sql.ErrNoRows) || err == nil && (disabled == 1 || tokenVersion != claims.TokenVersion) {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if err != nil {
			app.serverError(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey{}, user{ID: claims.UserID, Name: username, IsAdmin: isAdmin == 1})))
	})
}

func (app *App) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !currentUser(r).IsAdmin {
			httpx.Error(w, http.StatusForbidden, "admin required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (app *App) serveSPA(router chi.Router) {
	filesystem := http.Dir(app.config.StaticDir)
	router.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(r.URL.Path)
		file, err := filesystem.Open(strings.TrimPrefix(path, "/"))
		if err != nil {
			r.URL.Path = "/"
		} else {
			_ = file.Close()
		}
		http.StripPrefix("/", http.FileServer(filesystem)).ServeHTTP(w, r)
	})
}

func (app *App) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		app.logger.Info("http", "method", r.Method, "path", r.URL.Path)
	})
}

func (app *App) serverError(w http.ResponseWriter, err error) {
	app.logger.Error("server error", "error", err)
	httpx.Error(w, http.StatusInternalServerError, "internal error")
}

type userContextKey struct{}

type user struct {
	ID      int64
	Name    string
	IsAdmin bool
}

func currentUser(r *http.Request) user {
	value, _ := r.Context().Value(userContextKey{}).(user)
	return value
}

func jsonUnmarshal(data []byte, target any) error {
	return json.Unmarshal(data, target)
}
