package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type DingTalkAdapter struct{}

func init() { Register(DingTalkAdapter{}) }

func (DingTalkAdapter) Type() string { return "dingtalk" }

func (DingTalkAdapter) Send(ctx context.Context, endpoint, secret string, message Message) error {
	if secret != "" {
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		sign := dingTalkSign(timestamp, secret)
		separator := "?"
		if strings.Contains(endpoint, "?") {
			separator = "&"
		}
		endpoint += separator + url.Values{"timestamp": []string{timestamp}, "sign": []string{sign}}.Encode()
	}

	var payload any
	if message.ApproveURL != "" && message.RejectURL != "" {
		payload = map[string]any{
			"msgtype": "actionCard",
			"actionCard": map[string]any{
				"title":          message.Title(),
				"text":           "### " + message.Title() + "\n\n" + message.MarkdownBody(),
				"btnOrientation": "1",
				"btns": []map[string]string{
					{"title": message.ApproveLabel(), "actionURL": message.ApproveURL},
					{"title": message.RejectLabel(), "actionURL": message.RejectURL},
				},
			},
		}
	} else {
		payload = map[string]any{
			"msgtype":  "markdown",
			"markdown": map[string]string{"title": message.Title(), "text": "### " + message.Title() + "\n\n" + message.MarkdownBody()},
		}
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
		return fmt.Errorf("dingtalk response invalid: %s", string(body))
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("dingtalk error %d: %s", result.ErrCode, result.ErrMsg)
	}
	return nil
}

func dingTalkSign(timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "\n" + secret))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
