package server

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"deploy-platform/internal/notify"
	"deploy-platform/internal/platform/httpx"

	"github.com/go-chi/chi/v5"
)

type notificationChannelView struct {
	ID               int64    `json:"id"`
	UserID           int64    `json:"user_id"`
	Username         string   `json:"username"`
	RoleIDs          []int64  `json:"role_ids"`
	RoleNames        []string `json:"role_names"`
	Type             string   `json:"type"`
	Name             string   `json:"name"`
	WebhookURL       string   `json:"webhook_url"`
	SecretConfigured bool     `json:"secret_configured"`
	Enabled          bool     `json:"enabled"`
	TeamIDs          []int64  `json:"team_ids"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
}

type notificationChannelRequest struct {
	RoleIDs    []int64 `json:"role_ids"`
	Type       string  `json:"type"`
	Name       string  `json:"name"`
	WebhookURL string  `json:"webhook_url"`
	Secret     string  `json:"secret"`
	Enabled    bool    `json:"enabled"`
	TeamIDs    []int64 `json:"team_ids"`
}

func (app *App) listNotificationChannels(w http.ResponseWriter, r *http.Request) {
	rows, err := app.db.QueryContext(r.Context(), `
		SELECT nc.id, nc.user_id, u.username, nc.type, nc.name, nc.webhook_url,
		       nc.secret_encrypted != '', CAST(nc.enabled AS INTEGER),
		       nc.created_at, nc.updated_at
		FROM notification_channels nc
		JOIN users u ON u.id = nc.user_id
		ORDER BY u.username, nc.id`)
	if err != nil {
		app.serverError(w, err)
		return
	}
	channels := make([]notificationChannelView, 0)
	index := make(map[int64]int)
	for rows.Next() {
		var channel notificationChannelView
		var enabled int
		if err := rows.Scan(
			&channel.ID, &channel.UserID, &channel.Username, &channel.Type,
			&channel.Name, &channel.WebhookURL, &channel.SecretConfigured,
			&enabled, &channel.CreatedAt, &channel.UpdatedAt,
		); err != nil {
			_ = rows.Close()
			app.serverError(w, err)
			return
		}
		channel.Enabled = enabled == 1
		channel.TeamIDs = make([]int64, 0)
		channel.RoleIDs = make([]int64, 0)
		channel.RoleNames = make([]string, 0)
		index[channel.ID] = len(channels)
		channels = append(channels, channel)
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
		SELECT channel_id, team_id
		FROM notification_channel_teams
		ORDER BY team_id`)
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer teamRows.Close()
	for teamRows.Next() {
		var channelID, teamID int64
		if err := teamRows.Scan(&channelID, &teamID); err != nil {
			app.serverError(w, err)
			return
		}
		if position, exists := index[channelID]; exists {
			channels[position].TeamIDs = append(channels[position].TeamIDs, teamID)
		}
	}
	if err := teamRows.Err(); err != nil {
		app.serverError(w, err)
		return
	}
	if err := teamRows.Close(); err != nil {
		app.serverError(w, err)
		return
	}

	roleRows, err := app.db.QueryContext(r.Context(), `
		SELECT ncr.channel_id, r.id, r.name
		FROM notification_channel_roles ncr
		JOIN roles r ON r.id = ncr.role_id
		ORDER BY r.name`)
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer roleRows.Close()
	for roleRows.Next() {
		var channelID, roleID int64
		var roleName string
		if err := roleRows.Scan(&channelID, &roleID, &roleName); err != nil {
			app.serverError(w, err)
			return
		}
		if position, exists := index[channelID]; exists {
			channels[position].RoleIDs = append(channels[position].RoleIDs, roleID)
			channels[position].RoleNames = append(channels[position].RoleNames, roleName)
		}
	}
	if err := roleRows.Err(); err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, channels)
}

func (app *App) createNotificationChannel(w http.ResponseWriter, r *http.Request) {
	var request notificationChannelRequest
	if err := httpx.Decode(w, r, 8<<10, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	request.Type = strings.TrimSpace(request.Type)
	request.Name = strings.TrimSpace(request.Name)
	request.WebhookURL = strings.TrimSpace(request.WebhookURL)
	if err := app.validateNotificationChannel(r.Context(), request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	secretEncrypted, err := EncryptSecret(app.config.SecretEncryptionKey, request.Secret)
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
		INSERT INTO notification_channels(user_id, type, name, webhook_url, secret_encrypted, enabled)
		VALUES ((SELECT id FROM users WHERE CAST(is_admin AS INTEGER) = 1 ORDER BY id LIMIT 1), ?, ?, ?, ?, ?)`,
		request.Type, request.Name, request.WebhookURL, secretEncrypted, request.Enabled,
	)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, http.StatusConflict, "notification channel already exists")
			return
		}
		app.serverError(w, err)
		return
	}
	channelID, err := result.LastInsertId()
	if err != nil {
		app.serverError(w, err)
		return
	}
	if err := replaceNotificationChannelTeams(r.Context(), tx, channelID, request.TeamIDs); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := replaceNotificationChannelRoles(r.Context(), tx, channelID, request.RoleIDs); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": channelID})
}

func (app *App) updateNotificationChannel(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid notification channel id")
		return
	}
	var request notificationChannelRequest
	if err := httpx.Decode(w, r, 8<<10, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	request.Type = strings.TrimSpace(request.Type)
	request.Name = strings.TrimSpace(request.Name)
	request.WebhookURL = strings.TrimSpace(request.WebhookURL)
	if err := app.validateNotificationChannel(r.Context(), request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	var secretEncrypted string
	if request.Secret != "" {
		secretEncrypted, err = EncryptSecret(app.config.SecretEncryptionKey, request.Secret)
		if err != nil {
			app.serverError(w, err)
			return
		}
	}
	tx, err := app.db.BeginTx(r.Context(), nil)
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer tx.Rollback()
	var result sql.Result
	if secretEncrypted != "" {
		result, err = tx.ExecContext(r.Context(), `
			UPDATE notification_channels
			SET type = ?, name = ?, webhook_url = ?, secret_encrypted = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`,
			request.Type, request.Name, request.WebhookURL, secretEncrypted, request.Enabled, id,
		)
	} else {
		result, err = tx.ExecContext(r.Context(), `
			UPDATE notification_channels
			SET type = ?, name = ?, webhook_url = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`,
			request.Type, request.Name, request.WebhookURL, request.Enabled, id,
		)
	}
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, http.StatusConflict, "notification channel already exists")
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
		httpx.Error(w, http.StatusNotFound, "notification channel not found")
		return
	}
	if err := replaceNotificationChannelTeams(r.Context(), tx, id, request.TeamIDs); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := replaceNotificationChannelRoles(r.Context(), tx, id, request.RoleIDs); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"updated": true})
}

func (app *App) deleteNotificationChannel(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid notification channel id")
		return
	}
	result, err := app.db.ExecContext(r.Context(), `DELETE FROM notification_channels WHERE id = ?`, id)
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
		httpx.Error(w, http.StatusNotFound, "notification channel not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (app *App) validateNotificationChannel(ctx context.Context, request notificationChannelRequest) error {
	request.Type = strings.TrimSpace(request.Type)
	request.Name = strings.TrimSpace(request.Name)
	request.WebhookURL = strings.TrimSpace(request.WebhookURL)
	if request.Type == "" || request.Name == "" || request.WebhookURL == "" || len(request.TeamIDs) == 0 {
		return errors.New("type, name, webhook_url and team_ids are required")
	}
	if _, exists := notify.Get(request.Type); !exists {
		return errors.New("unsupported notification type")
	}
	var exists int
	for _, teamID := range request.TeamIDs {
		if err := app.db.QueryRowContext(ctx, `SELECT 1 FROM teams WHERE id = ?`, teamID).Scan(&exists); err != nil {
			return errors.New("team not found")
		}
	}
	for _, roleID := range request.RoleIDs {
		if err := app.db.QueryRowContext(ctx, `SELECT 1 FROM roles WHERE id = ?`, roleID).Scan(&exists); err != nil {
			return errors.New("role not found")
		}
	}
	var adminExists int
	if err := app.db.QueryRowContext(ctx, `SELECT 1 FROM users WHERE CAST(is_admin AS INTEGER) = 1 AND CAST(disabled AS INTEGER) = 0`).Scan(&adminExists); err != nil {
		return errors.New("active administrator is required")
	}
	return nil
}

func replaceNotificationChannelTeams(ctx context.Context, tx *sql.Tx, channelID int64, teamIDs []int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM notification_channel_teams WHERE channel_id = ?`, channelID); err != nil {
		return err
	}
	for _, teamID := range teamIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO notification_channel_teams(channel_id, team_id) VALUES (?, ?)`,
			channelID, teamID,
		); err != nil {
			return errors.New("team not found")
		}
	}
	return nil
}

func replaceNotificationChannelRoles(ctx context.Context, tx *sql.Tx, channelID int64, roleIDs []int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM notification_channel_roles WHERE channel_id = ?`, channelID); err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO notification_channel_roles(channel_id, role_id) VALUES (?, ?)`,
			channelID, roleID,
		); err != nil {
			return errors.New("role not found")
		}
	}
	return nil
}

func (app *App) notifyDeployment(event, projectName, version, status, operator, comment string, deploymentID, projectID int64, approvalToken string) {
	message := notify.Message{
		Event: event, Project: projectName, Version: version,
		Status: status, Operator: operator, Comment: comment,
	}
	if event == "deploy_requested" {
		message.ApproveURL = app.config.PublicURL + "/api/deployments/" + int64ToString(deploymentID) + "/approve?token=" + approvalToken
		message.RejectURL = app.config.PublicURL + "/api/deployments/" + int64ToString(deploymentID) + "/reject?token=" + approvalToken
	}
	go app.sendNotifications(message, projectID)
}

func (app *App) sendNotifications(message notify.Message, projectID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	rows, err := app.db.QueryContext(ctx, `
		SELECT DISTINCT nc.type, nc.webhook_url, nc.secret_encrypted
		FROM notification_channels nc
		JOIN users u ON u.id = nc.user_id
		JOIN projects p ON p.id = ?
		JOIN notification_channel_teams nct ON nct.channel_id = nc.id AND nct.team_id = p.team_id
		WHERE CAST(nc.enabled AS INTEGER) = 1
		  AND CAST(u.disabled AS INTEGER) = 0
		  AND (
		  	CAST(u.is_admin AS INTEGER) = 1
		  	OR (
		  		EXISTS (
		  			SELECT 1
		  			FROM user_projects up
		  			WHERE up.user_id = u.id AND up.project_id = p.id
		  		)
		  		AND EXISTS (
		  			SELECT 1
		  			FROM notification_channel_roles ncr
		  			JOIN user_roles ur ON ur.role_id = ncr.role_id
		  			JOIN role_permissions rp2 ON rp2.role_id = ur.role_id
		  			JOIN route_permissions rp ON rp.id = rp2.permission_id
		  			WHERE ncr.channel_id = nc.id
		  			  AND ur.user_id = u.id
		  			  AND rp.code = 'deploy:approve'
		  		)
		  	)
		  )`, projectID)
	if err != nil {
		app.logger.Error("notification query failed", "error", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var channelType, endpoint, secretEncrypted string
		if err := rows.Scan(&channelType, &endpoint, &secretEncrypted); err != nil {
			app.logger.Error("notification scan failed", "error", err)
			return
		}
		adapter, exists := notify.Get(channelType)
		if !exists {
			continue
		}
		secret := ""
		if secretEncrypted != "" {
			secret, err = DecryptSecret(app.config.SecretEncryptionKey, secretEncrypted)
			if err != nil {
				app.logger.Error("notification secret decrypt failed", "error", err)
				continue
			}
		}
		if err := adapter.Send(ctx, endpoint, secret, message); err != nil {
			app.logger.Error("notification send failed", "type", channelType, "error", err)
		}
	}
	if err := rows.Err(); err != nil {
		app.logger.Error("notification iteration failed", "error", err)
	}
}

func int64ToString(value int64) string {
	return strconv.FormatInt(value, 10)
}
