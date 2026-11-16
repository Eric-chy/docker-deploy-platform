package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"deploy-platform/internal/platform/httpx"

	"github.com/go-chi/chi/v5"
)

type deployRequest struct {
	ProjectID   int64  `json:"project_id"`
	ComposeYAML string `json:"compose_yaml"`
}

const deploymentPageSize = 50

func (app *App) deploy(w http.ResponseWriter, r *http.Request) {
	var request deployRequest
	if err := httpx.Decode(w, r, 1<<20, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(request.ComposeYAML) == "" {
		httpx.Error(w, http.StatusBadRequest, "compose_yaml is required")
		return
	}

	project, ok := app.canAccessProject(w, r, request.ProjectID, "deploy:publish")
	if !ok {
		return
	}

	snapshot, err := app.createDeploymentSnapshot(r.Context(), project, request.ComposeYAML, "deploy", "pending_approval", "pending", currentUser(r))
	if err != nil {
		app.serverError(w, err)
		return
	}
	app.notifyDeployment("deploy_requested", project.Name, snapshot.Version, snapshot.Status, currentUser(r).Name, "", snapshot.ID, project.ID, snapshot.ApprovalToken)
	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{
		"task_id": snapshot.TaskID,
		"version": snapshot.Version,
		"status":  snapshot.Status,
	})
}

func (app *App) createDeploymentSnapshot(
	ctx context.Context,
	targetProject project,
	composeYAML,
	action,
	status,
	approvalStatus string,
	operator user,
) (deploymentSnapshot, error) {
	taskID, err := randomID()
	if err != nil {
		return deploymentSnapshot{}, err
	}
	version, err := newDeploymentVersion()
	if err != nil {
		return deploymentSnapshot{}, err
	}
	approvalToken, err := randomID()
	if err != nil {
		return deploymentSnapshot{}, err
	}
	approvalTokenHashBytes := sha256.Sum256([]byte(approvalToken))
	snapshot := deploymentSnapshot{
		ProjectID:         targetProject.ID,
		Version:           version,
		TaskID:            taskID,
		Action:            action,
		BeforeYAML:        targetProject.CurrentYAML,
		AfterYAML:         composeYAML,
		Status:            status,
		ApprovalStatus:    approvalStatus,
		ApprovalToken:     approvalToken,
		ApprovalTokenHash: hex.EncodeToString(approvalTokenHashBytes[:]),
	}
	result, err := app.db.ExecContext(ctx, `
		INSERT INTO deploy_snapshots(
			task_id, project_id, version, action, operator_id, before_yaml,
			after_yaml, status, approval_status, approval_token_hash
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		taskID, targetProject.ID, version, action, operator.ID,
		targetProject.CurrentYAML, composeYAML, status, approvalStatus, snapshot.ApprovalTokenHash,
	)
	if err == nil {
		snapshot.ID, err = result.LastInsertId()
	}
	return snapshot, err
}

func (app *App) executeDeployment(ctx context.Context, targetProject project, snapshot deploymentSnapshot) error {
	node, err := app.getNode(ctx, targetProject.NodeID)
	if err != nil {
		app.markDeploymentFailed(ctx, snapshot.TaskID, err)
		return err
	}
	apiKey, err := DecryptSecret(app.config.SecretEncryptionKey, node.Secret)
	if err != nil {
		app.markDeploymentFailed(ctx, snapshot.TaskID, err)
		return err
	}

	nodeRequest := NodeDeployRequest{
		TaskID:      snapshot.TaskID,
		Project:     targetProject.Name,
		Version:     snapshot.Version,
		ComposeYAML: snapshot.AfterYAML,
		BeforeYAML:  snapshot.BeforeYAML,
		Action:      snapshot.Action,
	}
	_, err = NewNodeClient().Deploy(ctx, node.Address, apiKey, nodeRequest)
	if err != nil {
		_, updateErr := app.db.ExecContext(ctx, `UPDATE deploy_snapshots SET status = 'failed', output = ? WHERE task_id = ?`, err.Error(), snapshot.TaskID)
		if updateErr != nil {
			return updateErr
		}
		return err
	}
	return nil
}

func (app *App) markDeploymentFailed(ctx context.Context, taskID string, err error) {
	if _, updateErr := app.db.ExecContext(ctx, `UPDATE deploy_snapshots SET status = 'failed', output = ? WHERE task_id = ?`, err.Error(), taskID); updateErr != nil {
		app.logger.Error("mark deployment failed", "error", updateErr)
	}
}

func (app *App) startDeployment(w http.ResponseWriter, r *http.Request, targetProject project, composeYAML, action string) {
	snapshot, err := app.createDeploymentSnapshot(r.Context(), targetProject, composeYAML, action, "running", "", currentUser(r))
	if err != nil {
		app.serverError(w, err)
		return
	}
	if err := app.executeDeployment(r.Context(), targetProject, snapshot); err != nil {
		httpx.Error(w, http.StatusBadGateway, "node deploy failed")
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"task_id": snapshot.TaskID, "version": snapshot.Version})
}

type deploymentSnapshot struct {
	ID                int64
	ProjectID         int64
	Version           string
	TaskID            string
	Action            string
	BeforeYAML        string
	AfterYAML         string
	Status            string
	OperatorID        int64
	ApprovalStatus    string
	ApproverID        sql.NullInt64
	ApprovalComment   string
	ApprovalToken     string
	ApprovalTokenHash string
}

func (app *App) getDeploymentSnapshot(ctx context.Context, id int64) (deploymentSnapshot, error) {
	var snapshot deploymentSnapshot
	err := app.db.QueryRowContext(ctx, `
		SELECT id, project_id, version, task_id, action, before_yaml, after_yaml,
		       status, operator_id, approval_status, approver_id, approval_comment, approval_token_hash
		FROM deploy_snapshots
		WHERE id = ?`, id,
	).Scan(
		&snapshot.ID, &snapshot.ProjectID, &snapshot.Version, &snapshot.TaskID,
		&snapshot.Action, &snapshot.BeforeYAML, &snapshot.AfterYAML,
		&snapshot.Status, &snapshot.OperatorID, &snapshot.ApprovalStatus,
		&snapshot.ApproverID, &snapshot.ApprovalComment, &snapshot.ApprovalTokenHash,
	)
	return snapshot, err
}

type deploymentApprovalRequest struct {
	Comment string `json:"comment"`
}

func (app *App) approveDeployment(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	var request deploymentApprovalRequest
	if err := httpx.Decode(w, r, 4<<10, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	snapshot, err := app.getDeploymentSnapshot(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "deployment not found")
		return
	}
	if err != nil {
		app.serverError(w, err)
		return
	}
	if snapshot.Status != "pending_approval" || snapshot.ApprovalStatus != "pending" {
		httpx.Error(w, http.StatusConflict, "deployment is not waiting for approval")
		return
	}
	targetProject, ok := app.canAccessProject(w, r, snapshot.ProjectID, "deploy:approve")
	if !ok {
		return
	}

	approvalResult, err := app.db.ExecContext(r.Context(), `
		UPDATE deploy_snapshots
		SET status = 'running', approval_status = 'approved', approver_id = ?,
		    approval_comment = ?, approval_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = 'pending_approval'`,
		currentUser(r).ID, request.Comment, id,
	)
	if err != nil {
		app.serverError(w, err)
		return
	}
	approvalRows, err := approvalResult.RowsAffected()
	if err != nil {
		app.serverError(w, err)
		return
	}
	if approvalRows == 0 {
		httpx.Error(w, http.StatusConflict, "deployment approval is already processed")
		return
	}
	app.notifyDeployment("deploy_approved", targetProject.Name, snapshot.Version, "running", currentUser(r).Name, request.Comment, snapshot.ID, targetProject.ID, "")
	if err := app.executeDeployment(r.Context(), targetProject, snapshot); err != nil {
		app.notifyDeployment("deploy_failed", targetProject.Name, snapshot.Version, "failed", currentUser(r).Name, request.Comment, snapshot.ID, targetProject.ID, "")
		httpx.Error(w, http.StatusBadGateway, "node deploy failed")
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{
		"task_id": snapshot.TaskID,
		"version": snapshot.Version,
		"status":  "running",
	})
}

func (app *App) rejectDeployment(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	var request deploymentApprovalRequest
	if err := httpx.Decode(w, r, 4<<10, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	snapshot, err := app.getDeploymentSnapshot(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "deployment not found")
		return
	}
	if err != nil {
		app.serverError(w, err)
		return
	}
	if snapshot.Status != "pending_approval" || snapshot.ApprovalStatus != "pending" {
		httpx.Error(w, http.StatusConflict, "deployment is not waiting for approval")
		return
	}
	targetProject, ok := app.canAccessProject(w, r, snapshot.ProjectID, "deploy:approve")
	if !ok {
		return
	}
	rejectResult, err := app.db.ExecContext(r.Context(), `
		UPDATE deploy_snapshots
		SET status = 'rejected', approval_status = 'rejected', approver_id = ?,
		    approval_comment = ?, approval_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = 'pending_approval'`,
		currentUser(r).ID, request.Comment, id,
	)
	if err != nil {
		app.serverError(w, err)
		return
	}
	rejectRows, err := rejectResult.RowsAffected()
	if err != nil {
		app.serverError(w, err)
		return
	}
	if rejectRows == 0 {
		httpx.Error(w, http.StatusConflict, "deployment approval is already processed")
		return
	}
	app.notifyDeployment("deploy_rejected", targetProject.Name, snapshot.Version, "rejected", currentUser(r).Name, request.Comment, snapshot.ID, targetProject.ID, "")
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

func (app *App) approveDeploymentByToken(w http.ResponseWriter, r *http.Request) {
	id, snapshot, ok := app.deploymentFromApprovalToken(w, r)
	if !ok {
		return
	}
	targetProject, err := app.getProject(r.Context(), snapshot.ProjectID)
	if err != nil {
		app.serverError(w, err)
		return
	}
	approvalResult, err := app.db.ExecContext(r.Context(), `
		UPDATE deploy_snapshots
		SET status = 'running', approval_status = 'approved', approver_id = NULL,
		    approval_comment = '钉钉通知一键同意', approval_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = 'pending_approval'`, id,
	)
	if err != nil {
		app.serverError(w, err)
		return
	}
	approvalRows, err := approvalResult.RowsAffected()
	if err != nil {
		app.serverError(w, err)
		return
	}
	if approvalRows == 0 {
		writeApprovalHTML(w, http.StatusConflict, "无法处理", "该发布申请已处理。")
		return
	}
	app.notifyDeployment("deploy_approved", targetProject.Name, snapshot.Version, "running", "钉钉审批", "一键同意", snapshot.ID, targetProject.ID, "")
	if err := app.executeDeployment(r.Context(), targetProject, snapshot); err != nil {
		app.notifyDeployment("deploy_failed", targetProject.Name, snapshot.Version, "failed", "钉钉审批", "一键同意", snapshot.ID, targetProject.ID, "")
		writeApprovalHTML(w, http.StatusBadGateway, "发布失败", "节点执行失败，请到发布历史查看详情。")
		return
	}
	writeApprovalHTML(w, http.StatusOK, "发布已开始", "任务号："+snapshot.TaskID)
}

func (app *App) rejectDeploymentByToken(w http.ResponseWriter, r *http.Request) {
	id, snapshot, ok := app.deploymentFromApprovalToken(w, r)
	if !ok {
		return
	}
	targetProject, err := app.getProject(r.Context(), snapshot.ProjectID)
	if err != nil {
		app.serverError(w, err)
		return
	}
	rejectResult, err := app.db.ExecContext(r.Context(), `
		UPDATE deploy_snapshots
		SET status = 'rejected', approval_status = 'rejected', approver_id = NULL,
		    approval_comment = '钉钉通知一键拒绝', approval_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = 'pending_approval'`, id,
	)
	if err != nil {
		app.serverError(w, err)
		return
	}
	rejectRows, err := rejectResult.RowsAffected()
	if err != nil {
		app.serverError(w, err)
		return
	}
	if rejectRows == 0 {
		writeApprovalHTML(w, http.StatusConflict, "无法处理", "该发布申请已处理。")
		return
	}
	app.notifyDeployment("deploy_rejected", targetProject.Name, snapshot.Version, "rejected", "钉钉审批", "一键拒绝", snapshot.ID, targetProject.ID, "")
	writeApprovalHTML(w, http.StatusOK, "发布已拒绝", "任务号："+snapshot.TaskID)
}

func (app *App) deploymentFromApprovalToken(w http.ResponseWriter, r *http.Request) (int64, deploymentSnapshot, bool) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		writeApprovalHTML(w, http.StatusBadRequest, "链接无效", "发布记录 ID 无效。")
		return 0, deploymentSnapshot{}, false
	}
	token := r.URL.Query().Get("token")
	if len(token) < 16 {
		writeApprovalHTML(w, http.StatusUnauthorized, "链接无效", "审批令牌缺失或无效。")
		return 0, deploymentSnapshot{}, false
	}
	snapshot, err := app.getDeploymentSnapshot(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeApprovalHTML(w, http.StatusNotFound, "链接无效", "发布记录不存在。")
		return 0, deploymentSnapshot{}, false
	}
	if err != nil {
		app.serverError(w, err)
		return 0, deploymentSnapshot{}, false
	}
	tokenHash := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(tokenHash[:])), []byte(snapshot.ApprovalTokenHash)) != 1 {
		writeApprovalHTML(w, http.StatusUnauthorized, "链接无效", "审批令牌校验失败。")
		return 0, deploymentSnapshot{}, false
	}
	if snapshot.Status != "pending_approval" || snapshot.ApprovalStatus != "pending" {
		writeApprovalHTML(w, http.StatusConflict, "无法处理", "该发布申请已处理。")
		return 0, deploymentSnapshot{}, false
	}
	return id, snapshot, true
}

func writeApprovalHTML(w http.ResponseWriter, status int, title, detail string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>` + title + `</title><body style="font-family:system-ui;padding:48px;text-align:center"><h1>` + title + `</h1><p>` + detail + `</p></body></html>`))
}

func (app *App) rollbackDeployment(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	snapshot, err := app.getDeploymentSnapshot(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "deployment not found")
		return
	}
	if err != nil {
		app.serverError(w, err)
		return
	}
	if snapshot.Status == "running" || snapshot.Status == "pending" || snapshot.Status == "pending_approval" {
		httpx.Error(w, http.StatusConflict, "deployment is running")
		return
	}
	if strings.TrimSpace(snapshot.BeforeYAML) == "" {
		httpx.Error(w, http.StatusBadRequest, "deployment has no rollback configuration")
		return
	}
	targetProject, ok := app.canAccessProject(w, r, snapshot.ProjectID, "deploy:rollback")
	if !ok {
		return
	}
	app.startDeployment(w, r, targetProject, snapshot.BeforeYAML, "rollback")
}

func (app *App) retryDeployment(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	snapshot, err := app.getDeploymentSnapshot(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "deployment not found")
		return
	}
	if err != nil {
		app.serverError(w, err)
		return
	}
	if snapshot.Status == "success" {
		httpx.Error(w, http.StatusBadRequest, "successful deployment cannot be retried")
		return
	}
	if snapshot.Status == "running" || snapshot.Status == "pending" || snapshot.Status == "pending_approval" {
		httpx.Error(w, http.StatusConflict, "deployment is running")
		return
	}
	if strings.TrimSpace(snapshot.AfterYAML) == "" {
		httpx.Error(w, http.StatusBadRequest, "deployment has no publish configuration")
		return
	}
	targetProject, ok := app.canAccessProject(w, r, snapshot.ProjectID, "deploy:retry")
	if !ok {
		return
	}
	newSnapshot, err := app.createDeploymentSnapshot(r.Context(), targetProject, snapshot.AfterYAML, "deploy", "pending_approval", "pending", currentUser(r))
	if err != nil {
		app.serverError(w, err)
		return
	}
	app.notifyDeployment("deploy_requested", targetProject.Name, newSnapshot.Version, newSnapshot.Status, currentUser(r).Name, "", newSnapshot.ID, targetProject.ID, newSnapshot.ApprovalToken)
	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{
		"task_id": newSnapshot.TaskID,
		"version": newSnapshot.Version,
		"status":  newSnapshot.Status,
	})
}

type deployCallback struct {
	TaskID     string `json:"task_id"`
	Project    string `json:"project"`
	Version    string `json:"version"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	Output     string `json:"output"`
	DurationMS int64  `json:"duration_ms"`
}

func (app *App) deployCallback(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Callback-Token") != app.config.CallbackToken {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var callback deployCallback
	if err := httpx.Decode(w, r, 4<<20, &callback); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var projectID int64
	err := app.db.QueryRowContext(r.Context(), `SELECT project_id FROM deploy_snapshots WHERE task_id = ?`, callback.TaskID).Scan(&projectID)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		app.serverError(w, err)
		return
	}

	status := callback.Status
	if status != "success" && status != "failed" && status != "partial" && status != "failed_recovered" {
		httpx.Error(w, http.StatusBadRequest, "invalid status")
		return
	}
	result, err := app.db.ExecContext(r.Context(), `
		UPDATE deploy_snapshots
		SET status = ?, output = ?, duration_ms = ?
		WHERE task_id = ? AND status IN ('running', 'pending')`,
		status, callback.Message+"\n"+callback.Output, callback.DurationMS, callback.TaskID,
	)
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
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "already_processed"})
		return
	}
	if status == "success" {
		if _, err := app.db.ExecContext(r.Context(), `
			UPDATE projects
			SET current_yaml = deploy_snapshots.after_yaml,
			    current_version = ?,
			    updated_at = CURRENT_TIMESTAMP
			FROM deploy_snapshots
			WHERE projects.id = ? AND deploy_snapshots.task_id = ?`,
			callback.Version, projectID, callback.TaskID,
		); err != nil {
			app.serverError(w, err)
			return
		}
	}
	notificationEvent := "deploy_success"
	if status != "success" {
		notificationEvent = "deploy_failed"
	}
	app.notifyDeployment(notificationEvent, callback.Project, callback.Version, status, "", "", 0, projectID, "")
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (app *App) deployments(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "project_id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if _, ok := app.canAccessProject(w, r, id, "project:view"); !ok {
		return
	}
	page, err := parsePage(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid page")
		return
	}
	var total int64
	if err := app.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM deploy_snapshots WHERE project_id = ?`, id).Scan(&total); err != nil {
		app.serverError(w, err)
		return
	}
	rows, err := app.db.QueryContext(r.Context(), `
		SELECT d.id, d.task_id, d.version, d.action, d.status, d.created_at,
		       d.duration_ms, requester.username, approver.username,
		       d.operator_id, d.approval_status, d.approval_comment, d.approval_at
		FROM deploy_snapshots d
		LEFT JOIN users requester ON requester.id = d.operator_id
		LEFT JOIN users approver ON approver.id = d.approver_id
		WHERE d.project_id = ?
		ORDER BY d.created_at DESC
		LIMIT ? OFFSET ?`, id, deploymentPageSize, (page-1)*deploymentPageSize)
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer rows.Close()

	type deployment struct {
		ID              int64   `json:"id"`
		OperatorID      int64   `json:"operator_id"`
		TaskID          string  `json:"task_id"`
		Version         string  `json:"version"`
		Action          string  `json:"action"`
		Status          string  `json:"status"`
		CreatedAt       string  `json:"created_at"`
		DurationMS      int64   `json:"duration_ms"`
		Operator        *string `json:"operator"`
		Approver        *string `json:"approver"`
		ApprovalStatus  string  `json:"approval_status"`
		ApprovalComment string  `json:"approval_comment"`
		ApprovalAt      *string `json:"approval_at"`
	}
	deployments := make([]deployment, 0)
	for rows.Next() {
		var result deployment
		if err := rows.Scan(
			&result.ID, &result.TaskID, &result.Version, &result.Action,
			&result.Status, &result.CreatedAt, &result.DurationMS,
			&result.Operator, &result.Approver, &result.OperatorID,
			&result.ApprovalStatus, &result.ApprovalComment, &result.ApprovalAt,
		); err != nil {
			app.serverError(w, err)
			return
		}
		deployments = append(deployments, result)
	}
	if err := rows.Err(); err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, deploymentPage{
		Items: deployments, Total: total, Page: page, PageSize: deploymentPageSize,
	})
}

func (app *App) allDeployments(w http.ResponseWriter, r *http.Request) {
	operator := currentUser(r)
	page, err := parsePage(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid page")
		return
	}
	var total int64
	countQuery := `SELECT COUNT(*) FROM deploy_snapshots d JOIN projects p ON p.id = d.project_id`
	countArgs := make([]any, 0)
	if !operator.IsAdmin {
		countQuery += ` JOIN user_projects up ON up.project_id = p.id AND up.user_id = ?`
		countArgs = append(countArgs, operator.ID)
	}
	if err := app.db.QueryRowContext(r.Context(), countQuery, countArgs...).Scan(&total); err != nil {
		app.serverError(w, err)
		return
	}
	query := `
		SELECT d.id, d.task_id, p.name, d.version, d.action, d.status, d.created_at,
		       d.duration_ms, requester.username, approver.username,
		       d.operator_id, d.approval_status, d.approval_comment, d.approval_at
		FROM deploy_snapshots d
		JOIN projects p ON p.id = d.project_id
		LEFT JOIN users requester ON requester.id = d.operator_id
		LEFT JOIN users approver ON approver.id = d.approver_id
		`
	args := make([]any, 0)
	if !operator.IsAdmin {
		query += `JOIN user_projects up ON up.project_id = d.project_id AND up.user_id = ? `
		args = append(args, operator.ID)
	}
	query += `ORDER BY d.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, deploymentPageSize, (page-1)*deploymentPageSize)
	rows, err := app.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer rows.Close()

	type deploymentView struct {
		ID              int64   `json:"id"`
		OperatorID      int64   `json:"operator_id"`
		TaskID          string  `json:"task_id"`
		Project         string  `json:"project"`
		Version         string  `json:"version"`
		Action          string  `json:"action"`
		Status          string  `json:"status"`
		CreatedAt       string  `json:"created_at"`
		DurationMS      int64   `json:"duration_ms"`
		Operator        *string `json:"operator"`
		Approver        *string `json:"approver"`
		ApprovalStatus  string  `json:"approval_status"`
		ApprovalComment string  `json:"approval_comment"`
		ApprovalAt      *string `json:"approval_at"`
	}
	deployments := make([]deploymentView, 0)
	for rows.Next() {
		var deployment deploymentView
		if err := rows.Scan(
			&deployment.ID,
			&deployment.TaskID,
			&deployment.Project,
			&deployment.Version,
			&deployment.Action,
			&deployment.Status,
			&deployment.CreatedAt,
			&deployment.DurationMS,
			&deployment.Operator,
			&deployment.Approver,
			&deployment.OperatorID,
			&deployment.ApprovalStatus,
			&deployment.ApprovalComment,
			&deployment.ApprovalAt,
		); err != nil {
			app.serverError(w, err)
			return
		}
		deployments = append(deployments, deployment)
	}
	if err := rows.Err(); err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, deploymentPage{
		Items: deployments, Total: total, Page: page, PageSize: deploymentPageSize,
	})
}

func (app *App) deploymentDetail(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid deployment id")
		return
	}

	operator := currentUser(r)
	if !operator.IsAdmin {
		var exists int
		err = app.db.QueryRowContext(r.Context(), `
			SELECT 1
			FROM deploy_snapshots d
			JOIN user_projects up ON up.project_id = d.project_id AND up.user_id = ?
			WHERE d.id = ?`, operator.ID, id,
		).Scan(&exists)
		if err != nil {
			httpx.Error(w, http.StatusForbidden, "permission required")
			return
		}
	}

	var detail deploymentDetail
	err = app.db.QueryRowContext(r.Context(), `
		SELECT d.id, d.task_id, p.name, d.version, d.action, d.status,
		       d.created_at, d.duration_ms, d.output,
		       requester.username, approver.username,
		       d.approval_status, d.approval_comment, d.approval_at
		FROM deploy_snapshots d
		JOIN projects p ON p.id = d.project_id
		LEFT JOIN users requester ON requester.id = d.operator_id
		LEFT JOIN users approver ON approver.id = d.approver_id
		WHERE d.id = ?`, id,
	).Scan(
		&detail.ID,
		&detail.TaskID,
		&detail.Project,
		&detail.Version,
		&detail.Action,
		&detail.Status,
		&detail.CreatedAt,
		&detail.DurationMS,
		&detail.Output,
		&detail.Operator,
		&detail.Approver,
		&detail.ApprovalStatus,
		&detail.ApprovalComment,
		&detail.ApprovalAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "deployment not found")
		return
	}
	if err != nil {
		app.serverError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, detail)
}

type deploymentDetail struct {
	ID              int64   `json:"id"`
	TaskID          string  `json:"task_id"`
	Project         string  `json:"project"`
	Version         string  `json:"version"`
	Action          string  `json:"action"`
	Status          string  `json:"status"`
	CreatedAt       string  `json:"created_at"`
	DurationMS      int64   `json:"duration_ms"`
	Output          string  `json:"output"`
	Operator        *string `json:"operator"`
	Approver        *string `json:"approver"`
	ApprovalStatus  string  `json:"approval_status"`
	ApprovalComment string  `json:"approval_comment"`
	ApprovalAt      *string `json:"approval_at"`
}

type deploymentPage struct {
	Items    any   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

func parsePage(r *http.Request) (int, error) {
	value := r.URL.Query().Get("page")
	if value == "" {
		return 1, nil
	}
	page, err := strconv.Atoi(value)
	if err != nil || page < 1 || page > 100000 {
		return 0, errors.New("invalid page")
	}
	return page, nil
}

func (app *App) getProject(ctx context.Context, id int64) (project, error) {
	var result project
	err := app.db.QueryRowContext(ctx, `
		SELECT id, name, team_id, node_id, current_yaml, current_version, image_repo
		FROM projects WHERE id = ?`, id,
	).Scan(&result.ID, &result.Name, &result.TeamID, &result.NodeID, &result.CurrentYAML, &result.CurrentVersion, &result.ImageRepo)
	return result, err
}

func (app *App) getNode(ctx context.Context, id int64) (node, error) {
	var result node
	err := app.db.QueryRowContext(ctx, `
		SELECT id, name, address, secret_encrypted FROM nodes WHERE id = ?`, id,
	).Scan(&result.ID, &result.Name, &result.Address, &result.Secret)
	return result, err
}

func randomID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func newDeploymentVersion() (string, error) {
	suffix, err := randomID()
	if err != nil {
		return "", err
	}
	return time.Now().UTC().Format("v20060102t150405z") + suffix[:6], nil
}
