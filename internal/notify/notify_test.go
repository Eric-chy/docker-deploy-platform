package notify

import (
	"strings"
	"testing"
)

func TestMessageBodyExcludesActions(t *testing.T) {
	message := Message{
		Event:      "deploy_requested",
		Project:    "my-app",
		Version:    "v1",
		Status:     "pending_approval",
		Operator:   "admin",
		ApproveURL: "https://example.test/approve",
		RejectURL:  "https://example.test/reject",
	}

	if strings.Contains(message.Body(), "https://example.test") {
		t.Fatalf("body should not contain action links: %s", message.Body())
	}
	if strings.Count(message.Body(), "\n") < 4 {
		t.Fatalf("body should contain separate lines: %s", message.Body())
	}
	if !strings.Contains(message.MarkdownActions(), "[✅ 同意发布](https://example.test/approve)") {
		t.Fatalf("missing approve action: %s", message.MarkdownActions())
	}
	if !strings.Contains(message.MarkdownActions(), "[❌ 拒绝发布](https://example.test/reject)") {
		t.Fatalf("missing reject action: %s", message.MarkdownActions())
	}
	if !strings.Contains(message.Body(), "事件：发布审批申请") {
		t.Fatalf("event should be humanized: %s", message.Body())
	}
	if !strings.Contains(message.Body(), "状态：待审批") {
		t.Fatalf("status should be humanized: %s", message.Body())
	}
	if strings.Count(message.MarkdownBody(), "\n\n") < 4 {
		t.Fatalf("markdown body should use blank lines: %s", message.MarkdownBody())
	}
}
