package server

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"deploy-platform/internal/platform/httpx"

	"github.com/go-chi/chi/v5"
)

type permissionView struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type roleView struct {
	ID            int64            `json:"id"`
	Code          string           `json:"code"`
	Name          string           `json:"name"`
	PermissionIDs []int64          `json:"permission_ids"`
	Permissions   []permissionView `json:"permissions"`
}

type roleRequest struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	PermissionIDs []int64 `json:"permission_ids"`
}

type permissionRequest struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Path string `json:"path"`
}

func (app *App) listRoles(w http.ResponseWriter, r *http.Request) {
	rows, err := app.db.QueryContext(r.Context(), `
		SELECT id, code, name FROM roles ORDER BY id`)
	if err != nil {
		app.serverError(w, err)
		return
	}
	roles := make([]roleView, 0)
	index := make(map[int64]int)
	for rows.Next() {
		var role roleView
		if err := rows.Scan(&role.ID, &role.Code, &role.Name); err != nil {
			_ = rows.Close()
			app.serverError(w, err)
			return
		}
		role.PermissionIDs = make([]int64, 0)
		role.Permissions = make([]permissionView, 0)
		index[role.ID] = len(roles)
		roles = append(roles, role)
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

	permissionRows, err := app.db.QueryContext(r.Context(), `
		SELECT rp.role_id, p.id, p.code, p.name, p.path
		FROM role_permissions rp
		JOIN route_permissions p ON p.id = rp.permission_id
		ORDER BY p.id`)
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer permissionRows.Close()
	for permissionRows.Next() {
		var roleID int64
		var permission permissionView
		if err := permissionRows.Scan(&roleID, &permission.ID, &permission.Code, &permission.Name, &permission.Path); err != nil {
			app.serverError(w, err)
			return
		}
		if position, exists := index[roleID]; exists {
			roles[position].PermissionIDs = append(roles[position].PermissionIDs, permission.ID)
			roles[position].Permissions = append(roles[position].Permissions, permission)
		}
	}
	if err := permissionRows.Err(); err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, roles)
}

func (app *App) createRole(w http.ResponseWriter, r *http.Request) {
	var request roleRequest
	if err := httpx.Decode(w, r, 8<<10, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	request.Code = strings.TrimSpace(request.Code)
	request.Name = strings.TrimSpace(request.Name)
	if request.Code == "" || request.Name == "" || len(request.PermissionIDs) == 0 {
		httpx.Error(w, http.StatusBadRequest, "role code, name and permissions are required")
		return
	}

	tx, err := app.db.BeginTx(r.Context(), nil)
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO roles(code, name, permissions) VALUES (?, ?, ?)`,
		request.Code, request.Name, "[]",
	)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, http.StatusConflict, "role code or name already exists")
			return
		}
		app.serverError(w, err)
		return
	}
	roleID, err := result.LastInsertId()
	if err != nil {
		app.serverError(w, err)
		return
	}
	if err := replaceRolePermissions(r.Context(), tx, roleID, request.PermissionIDs); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"id":             roleID,
		"code":           request.Code,
		"name":           request.Name,
		"permission_ids": request.PermissionIDs,
	})
}

func (app *App) updateRole(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid role id")
		return
	}
	var request roleRequest
	if err := httpx.Decode(w, r, 8<<10, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	request.Code = strings.TrimSpace(request.Code)
	request.Name = strings.TrimSpace(request.Name)
	if request.Code == "" || request.Name == "" || len(request.PermissionIDs) == 0 {
		httpx.Error(w, http.StatusBadRequest, "role code, name and permissions are required")
		return
	}

	tx, err := app.db.BeginTx(r.Context(), nil)
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(r.Context(), `
		UPDATE roles SET code = ?, name = ?, permissions = ? WHERE id = ?`,
		request.Code, request.Name, "[]", id,
	)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, http.StatusConflict, "role code or name already exists")
			return
		}
		app.serverError(w, err)
		return
	}
	rows, err := result.RowsAffected()
	if err != nil {
		app.serverError(w, err)
		return
	}
	if rows == 0 {
		httpx.Error(w, http.StatusNotFound, "role not found")
		return
	}
	if err := replaceRolePermissions(r.Context(), tx, id, request.PermissionIDs); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := replaceRoleUserPermissions(r.Context(), tx, id); err != nil {
		app.serverError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"updated": true})
}

func (app *App) deleteRole(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid role id")
		return
	}
	var count int
	if err := app.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM user_roles WHERE role_id = ?`, id).Scan(&count); err != nil {
		app.serverError(w, err)
		return
	}
	if count > 0 {
		httpx.Error(w, http.StatusBadRequest, "role is assigned to users")
		return
	}
	result, err := app.db.ExecContext(r.Context(), `DELETE FROM roles WHERE id = ?`, id)
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
		httpx.Error(w, http.StatusNotFound, "role not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (app *App) listPermissions(w http.ResponseWriter, r *http.Request) {
	rows, err := app.db.QueryContext(r.Context(), `
		SELECT id, code, name, path FROM route_permissions ORDER BY id`)
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer rows.Close()
	permissions := make([]permissionView, 0)
	for rows.Next() {
		var permission permissionView
		if err := rows.Scan(&permission.ID, &permission.Code, &permission.Name, &permission.Path); err != nil {
			app.serverError(w, err)
			return
		}
		permissions = append(permissions, permission)
	}
	if err := rows.Err(); err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, permissions)
}

func (app *App) createPermission(w http.ResponseWriter, r *http.Request) {
	var request permissionRequest
	if err := httpx.Decode(w, r, 4<<10, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	request.Code = strings.TrimSpace(request.Code)
	request.Name = strings.TrimSpace(request.Name)
	request.Path = strings.TrimSpace(request.Path)
	if request.Code == "" || request.Name == "" || request.Path == "" {
		httpx.Error(w, http.StatusBadRequest, "permission code, name and path are required")
		return
	}
	result, err := app.db.ExecContext(r.Context(), `
		INSERT INTO route_permissions(code, name, path) VALUES (?, ?, ?)`,
		request.Code, request.Name, request.Path,
	)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, http.StatusConflict, "permission code already exists")
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
	httpx.WriteJSON(w, http.StatusCreated, permissionView{ID: id, Code: request.Code, Name: request.Name, Path: request.Path})
}

func (app *App) updatePermission(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid permission id")
		return
	}
	var request permissionRequest
	if err := httpx.Decode(w, r, 4<<10, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	request.Code = strings.TrimSpace(request.Code)
	request.Name = strings.TrimSpace(request.Name)
	request.Path = strings.TrimSpace(request.Path)
	if request.Code == "" || request.Name == "" || request.Path == "" {
		httpx.Error(w, http.StatusBadRequest, "permission code, name and path are required")
		return
	}
	result, err := app.db.ExecContext(r.Context(), `
		UPDATE route_permissions SET code = ?, name = ?, path = ? WHERE id = ?`,
		request.Code, request.Name, request.Path, id,
	)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, http.StatusConflict, "permission code already exists")
			return
		}
		app.serverError(w, err)
		return
	}
	rows, err := result.RowsAffected()
	if err != nil {
		app.serverError(w, err)
		return
	}
	if rows == 0 {
		httpx.Error(w, http.StatusNotFound, "permission not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"updated": true})
}

func (app *App) deletePermission(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid permission id")
		return
	}

	roleRows, err := app.db.QueryContext(r.Context(), `
		SELECT r.name
		FROM role_permissions rp
		JOIN roles r ON r.id = rp.role_id
		WHERE rp.permission_id = ?
		ORDER BY r.name`, id)
	if err != nil {
		app.serverError(w, err)
		return
	}
	roleNames := make([]string, 0)
	for roleRows.Next() {
		var roleName string
		if err := roleRows.Scan(&roleName); err != nil {
			_ = roleRows.Close()
			app.serverError(w, err)
			return
		}
		roleNames = append(roleNames, roleName)
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
	if len(roleNames) > 0 {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"error": "permission is assigned to roles; update role permissions first",
			"roles": roleNames,
		})
		return
	}
	result, err := app.db.ExecContext(r.Context(), `DELETE FROM route_permissions WHERE id = ?`, id)
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
		httpx.Error(w, http.StatusNotFound, "permission not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func replaceRoleUserPermissions(ctx context.Context, tx *sql.Tx, roleID int64) error {
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM user_permissions
		WHERE user_id IN (SELECT user_id FROM user_roles WHERE role_id = ?)`,
		roleID,
	); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO user_permissions(user_id, permission_id)
		SELECT DISTINCT ur.user_id, rp.permission_id
		FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id = ur.role_id
		WHERE ur.user_id IN (SELECT user_id FROM user_roles WHERE role_id = ?)`, roleID)
	return err
}

func replaceRolePermissions(ctx context.Context, tx *sql.Tx, roleID int64, permissionIDs []int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id = ?`, roleID); err != nil {
		return err
	}
	for _, permissionID := range permissionIDs {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO role_permissions(role_id, permission_id) VALUES (?, ?)`,
			roleID, permissionID,
		)
		if err != nil {
			return errors.New("permission not found")
		}
		if rows, err := result.RowsAffected(); err != nil || rows == 0 {
			return errors.New("permission not found")
		}
	}
	return nil
}
