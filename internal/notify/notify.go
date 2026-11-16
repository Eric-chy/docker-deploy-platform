package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

type Message struct {
	Event      string
	Project    string
	Version    string
	Status     string
	Operator   string
	Comment    string
	ApproveURL string
	RejectURL  string
}

func (message Message) Title() string {
	return fmt.Sprintf("[Docker发布] %s：%s", message.eventLabel(), message.Project)
}

func (message Message) Text() string {
	return strings.Join(message.bodyLines(), "\n")
}

func (message Message) Body() string {
	return strings.Join(message.bodyLines(), "\n")
}

func (message Message) MarkdownBody() string {
	return strings.Join(message.bodyLines(), "\n\n")
}

func (message Message) bodyLines() []string {
	statusLabels := map[string]string{
		"pending":          "等待处理",
		"pending_approval": "待审批",
		"running":          "发布中",
		"success":          "成功",
		"failed":           "失败",
		"partial":          "部分成功",
		"failed_recovered": "失败已恢复",
		"rejected":         "已拒绝",
		"approved":         "已同意",
	}
	statusLabel, exists := statusLabels[message.Status]
	if !exists {
		statusLabel = message.Status
	}
	lines := []string{
		fmt.Sprintf("事件：%s", message.eventLabel()),
		fmt.Sprintf("项目：%s", message.Project),
		fmt.Sprintf("版本：%s", message.Version),
		fmt.Sprintf("状态：%s", statusLabel),
		fmt.Sprintf("操作人：%s", message.Operator),
	}
	if message.Comment != "" {
		lines = append(lines, fmt.Sprintf("备注：%s", message.Comment))
	}
	return lines
}

func (message Message) eventLabel() string {
	eventLabels := map[string]string{
		"deploy_requested": "发布审批申请",
		"deploy_approved":  "发布审批通过",
		"deploy_rejected":  "发布审批拒绝",
		"deploy_success":   "发布成功",
		"deploy_failed":    "发布失败",
	}
	if label, exists := eventLabels[message.Event]; exists {
		return label
	}
	return message.Event
}

func (message Message) HasActions() bool {
	return message.ApproveURL != "" && message.RejectURL != ""
}

func (message Message) MarkdownActions() string {
	if !message.HasActions() {
		return ""
	}
	return fmt.Sprintf(
		"[✅ 同意发布](%s)  [❌ 拒绝发布](%s)",
		message.ApproveURL,
		message.RejectURL,
	)
}

type Adapter interface {
	Type() string
	Send(ctx context.Context, endpoint string, secret string, message Message) error
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Adapter{}
)

func Register(adapter Adapter) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[adapter.Type()] = adapter
}

func Get(adapterType string) (Adapter, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	adapter, exists := registry[adapterType]
	return adapter, exists
}

func postJSON(ctx context.Context, endpoint string, payload any) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(request)
}
