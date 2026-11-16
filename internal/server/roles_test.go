package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestDeletePermissionRequiresUnassignedRole(t *testing.T) {
	db, err := OpenDB(filepath.Join(t.TempDir(), "control.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`INSERT INTO roles(code, name, permissions) VALUES ('test', '测试', '[]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO role_permissions(role_id, permission_id)
		SELECT r.id, p.id
		FROM roles r
		JOIN route_permissions p ON p.code = 'project:view'
		WHERE r.code = 'test'`); err != nil {
		t.Fatal(err)
	}

	app := New(Config{}, db)
	request := requestWithRouteParam(http.MethodDelete, "/api/permissions/1", "id", "1", nil)
	recorder := httptest.NewRecorder()
	app.deletePermission(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code %d: %s", recorder.Code, recorder.Body.String())
	}
	var deleteResponse struct {
		Error string   `json:"error"`
		Roles []string `json:"roles"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &deleteResponse); err != nil {
		t.Fatal(err)
	}
	if deleteResponse.Error != "permission is assigned to roles; update role permissions first" {
		t.Fatalf("unexpected error: %q", deleteResponse.Error)
	}
	wantRoles := []string{"发布", "只读", "测试", "管理"}
	if fmt.Sprint(deleteResponse.Roles) != fmt.Sprint(wantRoles) {
		t.Fatalf("expected roles %v, got %v", wantRoles, deleteResponse.Roles)
	}

	var roleCount int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM role_permissions rp
		JOIN roles r ON r.id = rp.role_id
		WHERE rp.permission_id = 1 AND r.code = 'test'`).Scan(&roleCount); err != nil {
		t.Fatal(err)
	}
	if roleCount != 1 {
		t.Fatalf("expected role assignment to remain, got %d", roleCount)
	}

	var permissionCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM route_permissions WHERE id = 1`).Scan(&permissionCount); err != nil {
		t.Fatal(err)
	}
	if permissionCount != 1 {
		t.Fatalf("expected permission to remain, got %d", permissionCount)
	}
}

func TestUpdateRoleRefreshesUserPermissions(t *testing.T) {
	db, err := OpenDB(filepath.Join(t.TempDir(), "control.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`INSERT INTO roles(code, name, permissions) VALUES ('test', '测试', '[]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO roles(code, name, permissions) VALUES ('other', '其他角色', '[]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users(username, password_hash) VALUES ('test', 'hash')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO user_roles(user_id, role_id)
		SELECT u.id, r.id FROM users u JOIN roles r ON r.code = 'test'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO user_roles(user_id, role_id)
		SELECT u.id, r.id FROM users u JOIN roles r ON r.code = 'other'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO role_permissions(role_id, permission_id)
		SELECT r.id, p.id
		FROM roles r
		JOIN route_permissions p ON p.code = 'project:view'
		WHERE r.code = 'test'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO role_permissions(role_id, permission_id)
		SELECT r.id, p.id
		FROM roles r
		JOIN route_permissions p ON p.code = 'team:view'
		WHERE r.code = 'other'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO user_permissions(user_id, permission_id)
		SELECT u.id, p.id FROM users u JOIN route_permissions p ON p.code = 'project:view'`); err != nil {
		t.Fatal(err)
	}

	var roleID, deployPublishID int64
	if err := db.QueryRow(`SELECT id FROM roles WHERE code = 'test'`).Scan(&roleID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT id FROM route_permissions WHERE code = 'deploy:publish'`).Scan(&deployPublishID); err != nil {
		t.Fatal(err)
	}

	body := fmt.Sprintf(`{"code":"test","name":"测试","permission_ids":[%d]}`, deployPublishID)
	app := New(Config{}, db)
	request := requestWithRouteParam(http.MethodPut, "/api/roles/1", "id", strconv.FormatInt(roleID, 10), strings.NewReader(body))
	recorder := httptest.NewRecorder()
	app.updateRole(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code %d: %s", recorder.Code, recorder.Body.String())
	}

	permissionCodes := make([]string, 0, 2)
	rows, err := db.Query(`
		SELECT DISTINCT p.code
		FROM user_permissions up
		JOIN route_permissions p ON p.id = up.permission_id
		WHERE up.user_id = (SELECT id FROM users WHERE username = 'test')
		ORDER BY p.code`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var permissionCode string
		if err := rows.Scan(&permissionCode); err != nil {
			_ = rows.Close()
			t.Fatal(err)
		}
		permissionCodes = append(permissionCodes, permissionCode)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	want := []string{"deploy:publish", "team:view"}
	if fmt.Sprint(permissionCodes) != fmt.Sprint(want) {
		var userRoles, rolePermissions string
		_ = db.QueryRow(`
			SELECT GROUP_CONCAT(r.code)
			FROM user_roles ur JOIN roles r ON r.id = ur.role_id
			WHERE ur.user_id = (SELECT id FROM users WHERE username = 'test')`).Scan(&userRoles)
		_ = db.QueryRow(`
			SELECT GROUP_CONCAT(r.code || ':' || p.code)
			FROM role_permissions rp
			JOIN roles r ON r.id = rp.role_id
			JOIN route_permissions p ON p.id = rp.permission_id
			WHERE rp.role_id IN (SELECT role_id FROM user_roles ur WHERE ur.user_id = (SELECT id FROM users WHERE username = 'test'))`).Scan(&rolePermissions)
		t.Fatalf("expected user permissions %v, got %v; user roles=%q; role permissions=%q", want, permissionCodes, userRoles, rolePermissions)
	}
}

func requestWithRouteParam(method, url, key, value string, body io.Reader) *http.Request {
	request := httptest.NewRequest(method, url, body)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add(key, value)
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
}
