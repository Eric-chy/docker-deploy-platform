package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"
)

type FeishuAdapter struct{}

func init() { Register(FeishuAdapter{}) }

func (FeishuAdapter) Type() string { return "feishu" }

func (FeishuAdapter) Send(ctx context.Context, endpoint, secret string, message Message) error {
	elements := make([]map[string]any, 0, len(message.bodyLines())+2)
	for _, line := range message.bodyLines() {
		elements = append(elements, map[string]any{
			"tag":  "div",
			"text": map[string]string{"tag": "lark_md", "content": line},
		})
	}
	if message.HasActions() {
		elements = append(elements, map[string]any{
			"tag": "hr",
		}, map[string]any{
			"tag": "action",
			"actions": []map[string]any{
				{
					"tag":  "button",
					"type": "primary",
					"text": map[string]string{
						"tag":     "plain_text",
						"content": message.ApproveLabel(),
					},
					"url": message.ApproveURL,
				},
				{
					"tag":  "button",
					"type": "danger",
					"text": map[string]string{
						"tag":     "plain_text",
						"content": message.RejectLabel(),
					},
					"url": message.RejectURL,
				},
			},
		})
	}
	payload := map[string]any{
		"msg_type": "interactive",
		"card": map[string]any{
			"config": map[string]any{"wide_screen_mode": true},
			"header": map[string]any{
				"title": map[string]string{
					"tag":     "plain_text",
					"content": message.Title(),
				},
			},
			"elements": elements,
		},
	}
	if secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(timestamp + "\n" + secret))
		payload["timestamp"] = timestamp
		payload["sign"] = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	}
	response, err := postJSON(ctx, endpoint, payload)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("feishu response invalid: %s", string(body))
	}
	if result.Code != 0 {
		return fmt.Errorf("feishu error %d: %s", result.Code, result.Msg)
	}
	return nil
}
