package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

type WeComAdapter struct{}

func init() { Register(WeComAdapter{}) }

func (WeComAdapter) Type() string { return "wecom" }

func (WeComAdapter) Send(ctx context.Context, endpoint, secret string, message Message) error {
	content := "### " + message.Title() + "\n" + message.MarkdownBody()
	if actions := message.MarkdownActions(); actions != "" {
		content += "\n\n" + actions
	}
	payload := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]any{
			"content": content,
		},
	}
	response, err := postJSON(ctx, endpoint, payload)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("wecom response invalid: %s", string(body))
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("wecom error %d: %s", result.ErrCode, result.ErrMsg)
	}
	return nil
}
