package server

import (
	"context"
	"database/sql"
	"net/http"

	"deploy-platform/internal/platform/httpx"
)

const (
	RoleViewer   = "viewer"
	RoleOperator = "operator"
	RoleManager  = "manager"
)

func userHasProjectPermission(ctx context.Context, db *sql.DB, operator user, projectID int64, permission string) bool {
	if operator.IsAdmin {
		return true
	}

	var exists int
	if err := db.QueryRowContext(ctx, `SELECT 1 FROM user_projects WHERE user_id = ? AND project_id = ?`, operator.ID, projectID).Scan(&exists); err != nil {
		return false
	}

	rows, err := db.QueryContext(ctx, `
		SELECT p.code
		FROM user_permissions up
		JOIN route_permissions p ON p.id = up.permission_id
		WHERE up.user_id = ?`, operator.ID)
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var permissionCode string
		if err := rows.Scan(&permissionCode); err != nil {
			return false
		}
		if permissionCode == permission {
			return true
		}
	}
	return false
}

func (app *App) canAccessProject(w http.ResponseWriter, r *http.Request, projectID int64, permission string) (project, bool) {
	targetProject, err := app.getProject(r.Context(), projectID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "project not found")
		return targetProject, false
	}
	if !userHasProjectPermission(r.Context(), app.db, currentUser(r), targetProject.ID, permission) {
		httpx.Error(w, http.StatusForbidden, "permission required")
		return targetProject, false
	}
	return targetProject, true
}

func (app *App) canAccessTeam(w http.ResponseWriter, r *http.Request, teamID int64, permission string) bool {
	operator := currentUser(r)
	if operator.IsAdmin {
		return true
	}
	var projectID int64
	if err := app.db.QueryRowContext(r.Context(), `
		SELECT p.id
		FROM user_projects up
		JOIN projects p ON p.id = up.project_id
		WHERE up.user_id = ? AND p.team_id = ?
		LIMIT 1`, operator.ID, teamID).Scan(&projectID); err != nil {
		httpx.Error(w, http.StatusForbidden, "permission required")
		return false
	}
	if userHasProjectPermission(r.Context(), app.db, operator, projectID, permission) {
		return true
	}
	httpx.Error(w, http.StatusForbidden, "permission required")
	return false
}
