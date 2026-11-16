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
	Locale     string
}

func (message Message) Title() string {
	return fmt.Sprintf("%s %s: %s", message.productLabel(), message.eventLabel(), message.Project)
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
	labels := message.fieldLabels()
	separator := ": "
	if message.locale() == "zh-CN" {
		separator = "："
	}
	lines := []string{
		fmt.Sprintf("%s%s%s", labels["event"], separator, message.eventLabel()),
		fmt.Sprintf("%s%s%s", labels["project"], separator, message.Project),
		fmt.Sprintf("%s%s%s", labels["version"], separator, message.Version),
		fmt.Sprintf("%s%s%s", labels["status"], separator, message.statusLabel()),
		fmt.Sprintf("%s%s%s", labels["operator"], separator, message.Operator),
	}
	if message.Comment != "" {
		lines = append(lines, fmt.Sprintf("%s%s%s", labels["comment"], separator, message.Comment))
	}
	return lines
}

func (message Message) eventLabel() string {
	eventLabels := map[string]map[string]string{
		"deploy_requested": {"zh-CN": "发布审批申请", "en-US": "Deployment approval requested"},
		"deploy_approved":  {"zh-CN": "发布审批通过", "en-US": "Deployment approval approved"},
		"deploy_rejected":  {"zh-CN": "发布审批拒绝", "en-US": "Deployment approval rejected"},
		"deploy_success":   {"zh-CN": "发布成功", "en-US": "Deployment succeeded"},
		"deploy_failed":    {"zh-CN": "发布失败", "en-US": "Deployment failed"},
	}
	if labels, exists := eventLabels[message.Event]; exists {
		return labels[message.locale()]
	}
	return message.Event
}

func (message Message) statusLabel() string {
	statusLabels := map[string]map[string]string{
		"pending":          {"zh-CN": "等待处理", "en-US": "Pending"},
		"pending_approval": {"zh-CN": "待审批", "en-US": "Pending approval"},
		"running":          {"zh-CN": "发布中", "en-US": "Deploying"},
		"success":          {"zh-CN": "成功", "en-US": "Success"},
		"failed":           {"zh-CN": "失败", "en-US": "Failed"},
		"partial":          {"zh-CN": "部分成功", "en-US": "Partial"},
		"failed_recovered": {"zh-CN": "失败已恢复", "en-US": "Failed and recovered"},
		"rejected":         {"zh-CN": "已拒绝", "en-US": "Rejected"},
		"approved":         {"zh-CN": "已同意", "en-US": "Approved"},
	}
	if labels, exists := statusLabels[message.Status]; exists {
		return labels[message.locale()]
	}
	return message.Status
}

func (message Message) productLabel() string {
	if message.locale() == "en-US" {
		return "[Docker Deploy]"
	}
	return "[Docker发布]"
}

func (message Message) fieldLabels() map[string]string {
	if message.locale() == "en-US" {
		return map[string]string{
			"event": "Event", "project": "Project", "version": "Version",
			"status": "Status", "operator": "Operator", "comment": "Comment",
		}
	}
	return map[string]string{
		"event": "事件", "project": "项目", "version": "版本",
		"status": "状态", "operator": "操作人", "comment": "备注",
	}
}

func (message Message) locale() string {
	if message.Locale == "en-US" {
		return "en-US"
	}
	return "zh-CN"
}

func (message Message) ApproveLabel() string {
	if message.locale() == "en-US" {
		return "✅ Approve"
	}
	return "✅ 同意发布"
}

func (message Message) RejectLabel() string {
	if message.locale() == "en-US" {
		return "❌ Reject"
	}
	return "❌ 拒绝发布"
}

func (message Message) HasActions() bool {
	return message.ApproveURL != "" && message.RejectURL != ""
}

func (message Message) MarkdownActions() string {
	if !message.HasActions() {
		return ""
	}
	return fmt.Sprintf(
		"[%s](%s)  [%s](%s)",
		message.ApproveLabel(),
		message.ApproveURL,
		message.RejectLabel(),
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
