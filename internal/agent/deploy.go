package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"deploy-platform/internal/platform/httpx"
)

var projectPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

var ErrLocked = errors.New("project is locked")

type deployRequest struct {
	TaskID      string `json:"task_id"`
	Project     string `json:"project"`
	Version     string `json:"version"`
	ComposeYAML string `json:"compose_yaml"`
	BeforeYAML  string `json:"before_yaml"`
	Action      string `json:"action"`
}

type callbackPayload struct {
	TaskID     string `json:"task_id"`
	Project    string `json:"project"`
	Version    string `json:"version"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	Output     string `json:"output"`
	DurationMS int64  `json:"duration_ms"`
}

func (app *App) deploy(w http.ResponseWriter, r *http.Request) {
	var request deployRequest
	if err := httpx.Decode(w, r, 1<<20, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if !projectPattern.MatchString(request.Project) || !projectPattern.MatchString(request.Version) {
		httpx.Error(w, http.StatusBadRequest, "invalid project or version")
		return
	}

	if previous, loaded := app.tasks.Load(request.TaskID); loaded {
		httpx.WriteJSON(w, http.StatusOK, previous)
		return
	}

	lock, err := acquireLock(app.config.DataDir, request.Project)
	if err != nil {
		if errors.Is(err, ErrLocked) {
			httpx.Error(w, http.StatusConflict, "project is deploying")
			return
		}
		app.logger().Error("acquire lock", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	app.tasks.Store(request.TaskID, map[string]string{"status": "accepted", "task_id": request.TaskID})
	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "task_id": request.TaskID})

	go func() {
		defer releaseLock(lock)
		start := time.Now()
		status, message, output := app.runDeploy(r.Context(), request)
		result := map[string]string{"status": status, "task_id": request.TaskID, "message": message}
		app.tasks.Store(request.TaskID, result)
		app.sendCallback(request, status, message, output, time.Since(start).Milliseconds())
	}()
}

func (app *App) runDeploy(parent context.Context, request deployRequest) (string, string, string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), time.Duration(app.config.DeployTimeout)*time.Second)
	defer cancel()

	stackDir := filepath.Join(app.config.DataDir, "stacks", request.Project)
	versionsDir := filepath.Join(stackDir, "versions")
	if err := os.MkdirAll(versionsDir, 0o700); err != nil {
		return "failed", "create directories failed", err.Error()
	}

	afterPath := filepath.Join(versionsDir, request.Version+"_after.yml")
	if err := os.WriteFile(afterPath, []byte(request.ComposeYAML), 0o600); err != nil {
		return "failed", "write compose file failed", err.Error()
	}

	currentPath := filepath.Join(stackDir, "current.yml")
	output, err := app.compose(ctx, request.Project, afterPath, "pull")
	if err == nil {
		upOutput, upErr := app.compose(ctx, request.Project, afterPath, "up", "-d", "--remove-orphans")
		output += "\n" + upOutput
		if upErr != nil {
			err = upErr
		}
	}
	if err != nil {
		status, message, recovery := app.recover(ctx, request, currentPath)
		return status, message, output + "\n" + recovery
	}

	if err := atomicSymlink(currentPath, afterPath); err != nil {
		return "partial", "switch current.yml failed", output + "\n" + err.Error()
	}
	return "success", "", output
}

func (app *App) recover(ctx context.Context, request deployRequest, currentPath string) (string, string, string) {
	if _, err := os.Stat(currentPath); err != nil {
		return "failed", "deploy failed and no previous version exists", err.Error()
	}
	output, err := app.compose(ctx, request.Project, currentPath, "up", "-d", "--remove-orphans")
	if err != nil {
		return "partial", "deploy failed and recovery failed", output + "\n" + err.Error()
	}
	return "failed_recovered", "deploy failed; previous version recovered", output
}

func (app *App) compose(ctx context.Context, project, file string, args ...string) (string, error) {
	commandName := "docker"
	commandArgs := append([]string{"compose", "-p", project, "-f", file}, args...)
	if _, err := exec.LookPath("docker-compose"); err == nil {
		if _, pluginErr := exec.LookPath("docker"); pluginErr == nil {
			probe := exec.Command("docker", "compose", "version")
			if probeErr := probe.Run(); probeErr != nil {
				commandName = "docker-compose"
				commandArgs = append([]string{"-p", project, "-f", file}, args...)
			}
		} else {
			commandName = "docker-compose"
			commandArgs = append([]string{"-p", project, "-f", file}, args...)
		}
	}
	command := exec.CommandContext(ctx, commandName, commandArgs...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return stdout.String() + "\n" + stderr.String(), err
}

func (app *App) sendCallback(request deployRequest, status, message, output string, durationMS int64) {
	payload, err := json.Marshal(callbackPayload{
		TaskID:     request.TaskID,
		Project:    request.Project,
		Version:    request.Version,
		Status:     status,
		Message:    message,
		Output:     output,
		DurationMS: durationMS,
	})
	if err != nil {
		app.logger().Error("encode callback", "error", err)
		return
	}

	for attempt := 0; attempt < 5; attempt++ {
		httpRequest, err := http.NewRequest(http.MethodPost, strings.TrimSuffix(app.config.ControlURL, "/")+"/api/callback/deploy", bytes.NewReader(payload))
		if err != nil {
			app.logger().Error("create callback request", "error", err)
			return
		}
		httpRequest.Header.Set("Content-Type", "application/json")
		httpRequest.Header.Set("X-Callback-Token", app.config.CallbackToken)
		httpResponse, err := http.DefaultClient.Do(httpRequest)
		if err == nil {
			_ = httpResponse.Body.Close()
			if httpResponse.StatusCode >= 200 && httpResponse.StatusCode < 300 {
				return
			}
		}
		time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
	}
	app.logger().Error("callback retries exhausted", "task_id", request.TaskID)
}

func acquireLock(dataDir, project string) (*os.File, error) {
	locksDir := filepath.Join(dataDir, "locks")
	if err := os.MkdirAll(locksDir, 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(locksDir, project+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrLocked
		}
		return nil, err
	}
	return file, nil
}

func releaseLock(file *os.File) {
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	_ = file.Close()
}

func atomicSymlink(currentPath, target string) error {
	tempPath := currentPath + ".tmp"
	if err := os.Remove(tempPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	relativeTarget, err := filepath.Rel(filepath.Dir(currentPath), target)
	if err != nil {
		relativeTarget = target
	}
	if err := os.Symlink(relativeTarget, tempPath); err != nil {
		return err
	}
	if err := os.Rename(tempPath, currentPath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}

func (app *App) logger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}
