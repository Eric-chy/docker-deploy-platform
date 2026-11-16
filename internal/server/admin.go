package server

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"deploy-platform/internal/platform/httpx"

	"github.com/go-chi/chi/v5"
)

type createTeamRequest struct {
	Name string `json:"name"`
}

func (app *App) createTeam(w http.ResponseWriter, r *http.Request) {
	var request createTeamRequest
	if err := httpx.Decode(w, r, 4<<10, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "name is required")
		return
	}

	result, err := app.db.ExecContext(r.Context(), `INSERT INTO teams(name) VALUES (?)`, request.Name)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, http.StatusConflict, "team already exists")
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
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": id, "name": request.Name})
}

func (app *App) updateTeam(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid team id")
		return
	}
	var request createTeamRequest
	if err := httpx.Decode(w, r, 4<<10, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "name is required")
		return
	}
	result, err := app.db.ExecContext(r.Context(), `UPDATE teams SET name = ? WHERE id = ?`, request.Name, id)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, http.StatusConflict, "team already exists")
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
		httpx.Error(w, http.StatusNotFound, "team not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "name": request.Name})
}

func (app *App) deleteTeam(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid team id")
		return
	}
	var projectCount int
	if err := app.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM projects WHERE team_id = ?`, id).Scan(&projectCount); err != nil {
		app.serverError(w, err)
		return
	}
	if projectCount > 0 {
		httpx.Error(w, http.StatusConflict, "team has projects")
		return
	}
	result, err := app.db.ExecContext(r.Context(), `DELETE FROM teams WHERE id = ?`, id)
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
		httpx.Error(w, http.StatusNotFound, "team not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

type createNodeRequest struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	APIKey  string `json:"api_key"`
}

type updateNodeRequest struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	APIKey  string `json:"api_key"`
}

func (app *App) createNode(w http.ResponseWriter, r *http.Request) {
	var request createNodeRequest
	if err := httpx.Decode(w, r, 8<<10, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.Name == "" || request.Address == "" || len(request.APIKey) < 32 {
		httpx.Error(w, http.StatusBadRequest, "invalid node configuration")
		return
	}

	encrypted, err := EncryptSecret(app.config.SecretEncryptionKey, request.APIKey)
	if err != nil {
		app.serverError(w, err)
		return
	}
	result, err := app.db.ExecContext(r.Context(), `
		INSERT INTO nodes(name, address, secret_encrypted, status) VALUES (?, ?, ?, 'offline')`,
		request.Name, request.Address, encrypted,
	)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, http.StatusConflict, "node already exists")
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
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": id, "name": request.Name, "address": request.Address})
}

func (app *App) updateNode(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid node id")
		return
	}
	var request updateNodeRequest
	if err := httpx.Decode(w, r, 8<<10, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.Name == "" || request.Address == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid node configuration")
		return
	}
	if request.APIKey != "" && len(request.APIKey) < 32 {
		httpx.Error(w, http.StatusBadRequest, "invalid node api key")
		return
	}

	secretEncrypted := ""
	if request.APIKey != "" {
		secretEncrypted, err = EncryptSecret(app.config.SecretEncryptionKey, request.APIKey)
		if err != nil {
			app.serverError(w, err)
			return
		}
	}

	result, err := app.db.ExecContext(r.Context(), `
		UPDATE nodes
		SET name = ?, address = ?, secret_encrypted = COALESCE(NULLIF(?, ''), secret_encrypted), status = 'offline'
		WHERE id = ?`,
		request.Name, request.Address, secretEncrypted, id,
	)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, http.StatusConflict, "node already exists")
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
		httpx.Error(w, http.StatusNotFound, "node not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "name": request.Name, "address": request.Address})
}

func (app *App) deleteNode(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid node id")
		return
	}
	var projectCount int
	if err := app.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM projects WHERE node_id = ?`, id).Scan(&projectCount); err != nil {
		app.serverError(w, err)
		return
	}
	if projectCount > 0 {
		httpx.Error(w, http.StatusConflict, "node has projects")
		return
	}
	result, err := app.db.ExecContext(r.Context(), `DELETE FROM nodes WHERE id = ?`, id)
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
		httpx.Error(w, http.StatusNotFound, "node not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func isUniqueViolation(err error) bool {
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func (app *App) teams(w http.ResponseWriter, r *http.Request) {
	operator := currentUser(r)
	query := `SELECT id, name, created_at FROM teams`
	args := make([]any, 0)
	if !operator.IsAdmin {
		query += ` WHERE id IN (SELECT team_id FROM user_teams WHERE user_id = ?)`
		args = append(args, operator.ID)
	}
	query += ` ORDER BY name`
	rows, err := app.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		app.serverError(w, err)
		return
	}

	type teamRecord struct {
		ID        int64
		Name      string
		CreatedAt string
	}
	teamRecords := make([]teamRecord, 0)
	for rows.Next() {
		var team teamRecord
		if err := rows.Scan(&team.ID, &team.Name, &team.CreatedAt); err != nil {
			app.serverError(w, err)
			return
		}
		teamRecords = append(teamRecords, team)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		app.serverError(w, err)
		return
	}
	if closeErr != nil {
		app.serverError(w, closeErr)
		return
	}

	roles := make(map[int64]string, len(teamRecords))
	if !operator.IsAdmin {
		roleRows, roleErr := app.db.QueryContext(r.Context(), `SELECT team_id, role FROM user_teams WHERE user_id = ?`, operator.ID)
		if roleErr != nil {
			app.serverError(w, roleErr)
			return
		}
		for roleRows.Next() {
			var teamID int64
			var role string
			if err := roleRows.Scan(&teamID, &role); err != nil {
				_ = roleRows.Close()
				app.serverError(w, err)
				return
			}
			roles[teamID] = role
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
	}

	teams := make([]map[string]any, 0)
	for _, team := range teamRecords {
		role := "admin"
		if !operator.IsAdmin {
			role = roles[team.ID]
		}
		teams = append(teams, map[string]any{"id": team.ID, "name": team.Name, "created_at": team.CreatedAt, "role": role})
	}
	httpx.WriteJSON(w, http.StatusOK, teams)
}

func (app *App) nodes(w http.ResponseWriter, r *http.Request) {
	rows, err := app.db.QueryContext(r.Context(), `SELECT id, name, address, secret_encrypted FROM nodes ORDER BY name`)
	if err != nil {
		app.serverError(w, err)
		return
	}
	type nodeRecord struct {
		ID              int64
		Name            string
		Address         string
		EncryptedSecret string
	}
	nodeRecords := make([]nodeRecord, 0)
	for rows.Next() {
		var node nodeRecord
		if err := rows.Scan(&node.ID, &node.Name, &node.Address, &node.EncryptedSecret); err != nil {
			app.serverError(w, err)
			return
		}
		nodeRecords = append(nodeRecords, node)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		app.serverError(w, err)
		return
	}
	if closeErr != nil {
		app.serverError(w, closeErr)
		return
	}

	nodes := make([]map[string]any, 0)
	for _, node := range nodeRecords {
		apiKey, err := DecryptSecret(app.config.SecretEncryptionKey, node.EncryptedSecret)
		if err != nil {
			app.serverError(w, err)
			return
		}
		status := "offline"
		version := ""
		healthContext, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		if health, err := NewNodeClient().Health(healthContext, node.Address, apiKey); err == nil {
			status = "online"
			version = health.Version
		}
		cancel()
		if _, err := app.db.ExecContext(r.Context(), `UPDATE nodes SET status = ?, agent_version = ? WHERE id = ?`, status, version, node.ID); err != nil {
			app.serverError(w, err)
			return
		}
		nodes = append(nodes, map[string]any{"id": node.ID, "name": node.Name, "address": node.Address, "status": status})
	}
	httpx.WriteJSON(w, http.StatusOK, nodes)
}
