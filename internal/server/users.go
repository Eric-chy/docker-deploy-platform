package server

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"deploy-platform/internal/platform/httpx"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

type userTeamView struct {
	TeamID int64  `json:"team_id"`
	Name   string `json:"name"`
}

type userProjectView struct {
	ProjectID int64  `json:"project_id"`
	Name      string `json:"name"`
}

type userRoleView struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type userView struct {
	ID          int64             `json:"id"`
	Username    string            `json:"username"`
	IsAdmin     bool              `json:"is_admin"`
	Disabled    bool              `json:"disabled"`
	Teams       []userTeamView    `json:"teams"`
	Projects    []userProjectView `json:"projects"`
	Roles       []userRoleView    `json:"roles"`
	Permissions []permissionView  `json:"permissions"`
}

type createUserRequest struct {
	Username   string  `json:"username"`
	Password   string  `json:"password"`
	IsAdmin    bool    `json:"is_admin"`
	TeamIDs    []int64 `json:"team_ids"`
	ProjectIDs []int64 `json:"project_ids"`
	RoleIDs    []int64 `json:"role_ids"`
}

type updateUserRequest struct {
	Username   string  `json:"username"`
	Password   string  `json:"password"`
	IsAdmin    bool    `json:"is_admin"`
	Disabled   bool    `json:"disabled"`
	TeamIDs    []int64 `json:"team_ids"`
	ProjectIDs []int64 `json:"project_ids"`
	RoleIDs    []int64 `json:"role_ids"`
}

func validPasswordFormat(password string) bool {
	if len(password) < 8 || len(password) > 18 {
		return false
	}

	var hasDigit, hasLowercase, hasUppercase, hasSymbol bool
	for _, char := range password {
		switch {
		case char >= '0' && char <= '9':
			hasDigit = true
		case char >= 'a' && char <= 'z':
			hasLowercase = true
		case char >= 'A' && char <= 'Z':
			hasUppercase = true
		case (char >= '!' && char <= '/') ||
			(char >= ':' && char <= '@') ||
			(char >= '[' && char <= '`') ||
			(char >= '{' && char <= '~'):
			hasSymbol = true
		default:
			return false
		}
	}

	categories := 0
	for _, exists := range []bool{hasDigit, hasLowercase, hasUppercase, hasSymbol} {
		if exists {
			categories++
		}
	}
	return categories >= 2
}

func (app *App) listUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := app.db.QueryContext(r.Context(), `
		SELECT id, username, CAST(is_admin AS INTEGER), CAST(disabled AS INTEGER)
		FROM users ORDER BY username`)
	if err != nil {
		app.serverError(w, err)
		return
	}
	users := make([]userView, 0)
	index := make(map[int64]int)
	for rows.Next() {
		var user userView
		var isAdmin, disabled int
		if err := rows.Scan(&user.ID, &user.Username, &isAdmin, &disabled); err != nil {
			_ = rows.Close()
			app.serverError(w, err)
			return
		}
		user.IsAdmin = isAdmin == 1
		user.Disabled = disabled == 1
		user.Teams = make([]userTeamView, 0)
		user.Projects = make([]userProjectView, 0)
		user.Roles = make([]userRoleView, 0)
		user.Permissions = make([]permissionView, 0)
		index[user.ID] = len(users)
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		app.serverError(w, err)
		return
	}
	if err := rows.Close(); err != nil {
		app.serverError(w, err)
		return
	}

	teamRows, err := app.db.QueryContext(r.Context(), `
		SELECT ut.user_id, ut.team_id, t.name
		FROM user_teams ut
		JOIN teams t ON t.id = ut.team_id
		ORDER BY t.name`)
	if err != nil {
		app.serverError(w, err)
		return
	}
	for teamRows.Next() {
		var userID int64
		var team userTeamView
		if err := teamRows.Scan(&userID, &team.TeamID, &team.Name); err != nil {
			app.serverError(w, err)
			return
		}
		if position, exists := index[userID]; exists {
			users[position].Teams = append(users[position].Teams, team)
		}
	}
	if err := teamRows.Err(); err != nil {
		_ = teamRows.Close()
		app.serverError(w, err)
		return
	}
	if err := teamRows.Close(); err != nil {
		app.serverError(w, err)
		return
	}

	projectRows, err := app.db.QueryContext(r.Context(), `
		SELECT up.user_id, p.id, p.name
		FROM user_projects up
		JOIN projects p ON p.id = up.project_id
		ORDER BY p.name`)
	if err != nil {
		app.serverError(w, err)
		return
	}
	for projectRows.Next() {
		var userID int64
		var project userProjectView
		if err := projectRows.Scan(&userID, &project.ProjectID, &project.Name); err != nil {
			_ = projectRows.Close()
			app.serverError(w, err)
			return
		}
		if position, exists := index[userID]; exists {
			users[position].Projects = append(users[position].Projects, project)
		}
	}
	if err := projectRows.Err(); err != nil {
		_ = projectRows.Close()
		app.serverError(w, err)
		return
	}
	if err := projectRows.Close(); err != nil {
		app.serverError(w, err)
		return
	}

	roleRows, err := app.db.QueryContext(r.Context(), `
		SELECT ur.user_id, r.id, r.code, r.name
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		ORDER BY r.name`)
	if err != nil {
		app.serverError(w, err)
		return
	}
	for roleRows.Next() {
		var userID int64
		var role userRoleView
		if err := roleRows.Scan(&userID, &role.ID, &role.Code, &role.Name); err != nil {
			_ = roleRows.Close()
			app.serverError(w, err)
			return
		}
		if position, exists := index[userID]; exists {
			users[position].Roles = append(users[position].Roles, role)
		}
	}
	if err := roleRows.Err(); err != nil {
		_ = roleRows.Close()
		app.serverError(w, err)
		return
	}
	if err := roleRows.Close(); err != nil {
		app.serverError(w, err)
		return
	}

	permissionRows, err := app.db.QueryContext(r.Context(), `
		SELECT up.user_id, p.id, p.code, p.name, p.path
		FROM user_permissions up
		JOIN route_permissions p ON p.id = up.permission_id
		ORDER BY p.id`)
	if err != nil {
		app.serverError(w, err)
		return
	}
	for permissionRows.Next() {
		var userID int64
		var permission permissionView
		if err := permissionRows.Scan(&userID, &permission.ID, &permission.Code, &permission.Name, &permission.Path); err != nil {
			_ = permissionRows.Close()
			app.serverError(w, err)
			return
		}
		if position, exists := index[userID]; exists {
			users[position].Permissions = append(users[position].Permissions, permission)
		}
	}
	if err := permissionRows.Err(); err != nil {
		_ = permissionRows.Close()
		app.serverError(w, err)
		return
	}
	if err := permissionRows.Close(); err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, users)
}

func (app *App) createUser(w http.ResponseWriter, r *http.Request) {
	var request createUserRequest
	if err := httpx.Decode(w, r, 16<<10, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	if request.Username == "" || !validPasswordFormat(request.Password) {
		httpx.Error(w, http.StatusBadRequest, "username is invalid or password must combine two character types")
		return
	}
	if !request.IsAdmin && len(request.ProjectIDs) == 0 {
		httpx.Error(w, http.StatusBadRequest, "non-admin user requires at least one project")
		return
	}
	if !request.IsAdmin && len(request.RoleIDs) == 0 {
		httpx.Error(w, http.StatusBadRequest, "non-admin user requires at least one role")
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		app.serverError(w, err)
		return
	}
	tx, err := app.db.BeginTx(r.Context(), nil)
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO users(username, password_hash, is_admin) VALUES (?, ?, ?)`,
		request.Username, string(passwordHash), request.IsAdmin,
	)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, http.StatusConflict, "user already exists")
			return
		}
		app.serverError(w, err)
		return
	}
	userID, err := result.LastInsertId()
	if err != nil {
		app.serverError(w, err)
		return
	}
	if err := replaceUserProjects(r.Context(), tx, userID, request.ProjectIDs); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := replaceUserRoles(r.Context(), tx, userID, request.RoleIDs); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := replaceUserPermissions(r.Context(), tx, userID); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": userID, "username": request.Username})
}

func (app *App) updateUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var request updateUserRequest
	if err := httpx.Decode(w, r, 16<<10, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	if request.Username == "" {
		httpx.Error(w, http.StatusBadRequest, "username is required")
		return
	}
	if request.Password != "" && !validPasswordFormat(request.Password) {
		httpx.Error(w, http.StatusBadRequest, "password must combine two character types")
		return
	}
	if !request.IsAdmin && len(request.ProjectIDs) == 0 {
		httpx.Error(w, http.StatusBadRequest, "non-admin user requires at least one project")
		return
	}
	if !request.IsAdmin && len(request.RoleIDs) == 0 {
		httpx.Error(w, http.StatusBadRequest, "non-admin user requires at least one role")
		return
	}
	operator := currentUser(r)
	if operator.ID == id && (!request.IsAdmin || request.Disabled) {
		httpx.Error(w, http.StatusBadRequest, "cannot remove your own admin access")
		return
	}

	tx, err := app.db.BeginTx(r.Context(), nil)
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer tx.Rollback()

	if request.Password == "" {
		_, err = tx.ExecContext(r.Context(), `
			UPDATE users
			SET username = ?, is_admin = ?, disabled = ?, token_version = token_version + 1
			WHERE id = ?`,
			request.Username, request.IsAdmin, request.Disabled, id,
		)
	} else {
		passwordHash, hashErr := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
		if hashErr != nil {
			app.serverError(w, hashErr)
			return
		}
		_, err = tx.ExecContext(r.Context(), `
			UPDATE users
			SET username = ?, password_hash = ?, is_admin = ?, disabled = ?, token_version = token_version + 1
			WHERE id = ?`,
			request.Username, string(passwordHash), request.IsAdmin, request.Disabled, id,
		)
	}
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, http.StatusConflict, "user already exists")
			return
		}
		app.serverError(w, err)
		return
	}
	if err := replaceUserProjects(r.Context(), tx, id, request.ProjectIDs); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := replaceUserRoles(r.Context(), tx, id, request.RoleIDs); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := replaceUserPermissions(r.Context(), tx, id); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"updated": true})
}

func (app *App) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if currentUser(r).ID == id {
		httpx.Error(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}
	result, err := app.db.ExecContext(r.Context(), `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		app.serverError(w, err)
		return
	}
	rows, err := result.RowsAffected()
	if err != nil {
		app.serverError(w, err)
		return
	}
	if rows == 0 {
		httpx.Error(w, http.StatusNotFound, "user not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func replaceUserTeams(ctx context.Context, tx *sql.Tx, userID int64, teamIDs []int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_teams WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, teamID := range teamIDs {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO user_teams(user_id, team_id, role)
			VALUES (?, ?, ?)`,
			userID, teamID, RoleViewer,
		)
		if err != nil {
			return err
		}
		if rows, err := result.RowsAffected(); err != nil || rows == 0 {
			return errors.New("team not found")
		}
	}
	return nil
}

func replaceUserProjects(ctx context.Context, tx *sql.Tx, userID int64, projectIDs []int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_projects WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, projectID := range projectIDs {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO user_projects(user_id, project_id) VALUES (?, ?)`,
			userID, projectID,
		)
		if err != nil {
			return errors.New("project not found")
		}
		if rows, err := result.RowsAffected(); err != nil || rows == 0 {
			return errors.New("project not found")
		}
	}
	return nil
}

func replaceUserRoles(ctx context.Context, tx *sql.Tx, userID int64, roleIDs []int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO user_roles(user_id, role_id) VALUES (?, ?)`,
			userID, roleID,
		)
		if err != nil {
			return errors.New("role not found")
		}
		if rows, err := result.RowsAffected(); err != nil || rows == 0 {
			return errors.New("role not found")
		}
	}
	return nil
}

func replaceUserPermissions(ctx context.Context, tx *sql.Tx, userID int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_permissions WHERE user_id = ?`, userID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO user_permissions(user_id, permission_id)
		SELECT DISTINCT ?, rp.permission_id
		FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id = ur.role_id
		WHERE ur.user_id = ?`, userID, userID)
	return err
}

func (app *App) userRolesAndPermissions(ctx context.Context, userID int64, isAdmin bool) ([]string, []string, error) {
	if isAdmin {
		return []string{"admin"}, []string{"*:*:*"}, nil
	}

	rows, err := app.db.QueryContext(ctx, `
		SELECT r.code
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = ?
		ORDER BY r.code`, userID)
	if err != nil {
		return nil, nil, err
	}
	roles := make([]string, 0)
	permissions := make([]string, 0)
	seenRoles := make(map[string]bool)
	seenPermissions := make(map[string]bool)
	for rows.Next() {
		var roleCode string
		if err := rows.Scan(&roleCode); err != nil {
			return nil, nil, err
		}
		if !seenRoles[roleCode] {
			seenRoles[roleCode] = true
			roles = append(roles, roleCode)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, nil, err
	}

	permissionRows, err := app.db.QueryContext(ctx, `
		SELECT p.code
		FROM user_permissions up
		JOIN route_permissions p ON p.id = up.permission_id
		WHERE up.user_id = ?
		ORDER BY p.id`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer permissionRows.Close()
	for permissionRows.Next() {
		var permission string
		if err := permissionRows.Scan(&permission); err != nil {
			return nil, nil, err
		}
		if !seenPermissions[permission] {
			seenPermissions[permission] = true
			permissions = append(permissions, permission)
		}
	}
	if err := permissionRows.Err(); err != nil {
		return nil, nil, err
	}
	return roles, permissions, nil
}
