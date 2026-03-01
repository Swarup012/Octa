// Octa - Personal AI Agent
// License: MIT

package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// EmailScheduler is the interface GmailTool needs for scheduled emails.
// Implemented by *scheduler.EmailQueue.
type EmailScheduler interface {
	Enqueue(ctx context.Context, to, subject, body string) error
	GetPendingCount(ctx context.Context) (int, error)
}

// GmailTool implements the Tool interface for Gmail.
// The *gmail.Service is created lazily on first use via sync.Once.
type GmailTool struct {
	httpClient *http.Client
	scheduler  EmailScheduler

	once sync.Once
	svc  *gmail.Service
}

// NewGmailTool creates a GmailTool. Service is lazy.
func NewGmailTool(httpClient *http.Client) *GmailTool {
	return &GmailTool{httpClient: httpClient}
}

// SetScheduler wires the email scheduler for schedule/list/cancel actions.
func (t *GmailTool) SetScheduler(s EmailScheduler) {
	t.scheduler = s
}

func (t *GmailTool) Name() string { return "gmail" }

func (t *GmailTool) Description() string {
	return "Send, search, and read Gmail emails. Actions: send, list_inbox, read, search, schedule, list_scheduled, cancel_scheduled."
}

func (t *GmailTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action":     map[string]any{"type": "string", "enum": []string{"send", "list_inbox", "read", "search", "schedule", "list_scheduled", "cancel_scheduled"}},
			"to":         map[string]any{"type": "string", "description": "Recipient email address(es), comma-separated"},
			"subject":    map[string]any{"type": "string", "description": "Email subject"},
			"body":       map[string]any{"type": "string", "description": "Email body"},
			"cc":         map[string]any{"type": "string", "description": "CC email address(es), comma-separated"},
			"is_html":    map[string]any{"type": "boolean", "description": "True if body is HTML"},
			"message_id": map[string]any{"type": "string", "description": "Gmail message ID for read action"},
			"query":      map[string]any{"type": "string", "description": "Gmail search query"},
			"count":      map[string]any{"type": "integer", "description": "Number of messages to return (default 10)"},
			"send_at":    map[string]any{"type": "string", "description": "Schedule time for schedule action (RFC3339 or 'YYYY-MM-DD HH:MM')"},
			"email_id":   map[string]any{"type": "integer", "description": "Scheduled email ID for cancel_scheduled"},
		},
		"required": []string{"action"},
	}
}

// service returns the cached *gmail.Service, lazy-initialised via sync.Once.
func (t *GmailTool) service() (*gmail.Service, error) {
	t.once.Do(func() {
		svc, err := gmail.NewService(context.Background(), option.WithHTTPClient(t.httpClient))
		if err != nil {
			return
		}
		t.svc = svc
	})
	if t.svc == nil {
		return nil, fmt.Errorf("gmail: service failed to initialise — check OAuth token")
	}
	return t.svc, nil
}

// SendImmediate sends an email immediately. Called by the dispatcher and send action.
func (t *GmailTool) SendImmediate(ctx context.Context, to []string, subject, body string, isHTML bool, cc []string) error {
	svc, err := t.service()
	if err != nil {
		return err
	}

	contentType := "text/plain"
	if isHTML {
		contentType = "text/html"
	}

	toStr := strings.Join(to, ", ")
	ccStr := strings.Join(cc, ", ")

	var raw strings.Builder
	raw.WriteString("To: " + toStr + "\r\n")
	if ccStr != "" {
		raw.WriteString("Cc: " + ccStr + "\r\n")
	}
	raw.WriteString("Subject: " + subject + "\r\n")
	raw.WriteString("Content-Type: " + contentType + "; charset=utf-8\r\n")
	raw.WriteString("\r\n")
	raw.WriteString(body)

	encoded := base64.URLEncoding.EncodeToString([]byte(raw.String()))
	msg := &gmail.Message{Raw: encoded}

	_, err = svc.Users.Messages.Send("me", msg).Context(ctx).Do()
	return err
}

func (t *GmailTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, _ := args["action"].(string)
	switch action {
	case "send":
		return t.send(ctx, args)
	case "list_inbox":
		return t.listInbox(ctx, args)
	case "read":
		return t.read(ctx, args)
	case "search":
		return t.search(ctx, args)
	case "schedule":
		return t.schedule(ctx, args)
	case "list_scheduled":
		return t.listScheduled(ctx)
	case "cancel_scheduled":
		return t.cancelScheduled(ctx, args)
	default:
		return ErrorResult(fmt.Sprintf("gmail: unknown action %q", action))
	}
}

func (t *GmailTool) send(ctx context.Context, args map[string]any) *ToolResult {
	toStr, _ := args["to"].(string)
	subject, _ := args["subject"].(string)
	body, _ := args["body"].(string)
	if toStr == "" || subject == "" || body == "" {
		return ErrorResult("gmail send: 'to', 'subject', and 'body' are required")
	}

	to := strings.Split(toStr, ",")
	for i := range to {
		to[i] = strings.TrimSpace(to[i])
	}

	var cc []string
	if ccStr, ok := args["cc"].(string); ok && ccStr != "" {
		cc = strings.Split(ccStr, ",")
		for i := range cc {
			cc[i] = strings.TrimSpace(cc[i])
		}
	}

	isHTML, _ := args["is_html"].(bool)

	if err := t.SendImmediate(ctx, to, subject, body, isHTML, cc); err != nil {
		return ErrorResult(fmt.Sprintf("gmail send: %v", err))
	}

	msg := fmt.Sprintf("✅ Email sent to %s — Subject: %q", toStr, subject)
	return &ToolResult{ForLLM: msg, ForUser: msg}
}

func (t *GmailTool) listInbox(ctx context.Context, args map[string]any) *ToolResult {
	svc, err := t.service()
	if err != nil {
		return ErrorResult(err.Error())
	}

	count := int64(10)
	if c, ok := args["count"].(float64); ok && c > 0 {
		count = int64(c)
	}

	resp, err := svc.Users.Messages.List("me").
		LabelIds("INBOX").
		MaxResults(count).
		Context(ctx).Do()
	if err != nil {
		return ErrorResult(fmt.Sprintf("gmail list_inbox: %v", err))
	}

	if len(resp.Messages) == 0 {
		msg := "📬 Inbox is empty."
		return &ToolResult{ForLLM: msg, ForUser: msg}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📬 Inbox (%d messages):\n\n", len(resp.Messages)))
	for i, m := range resp.Messages {
		full, err := svc.Users.Messages.Get("me", m.Id).
			Format("metadata").
			MetadataHeaders("Subject", "From", "Date").
			Context(ctx).Do()
		if err != nil {
			continue
		}
		subject, from, date := extractHeaders(full)
		sb.WriteString(fmt.Sprintf("%d. From: %s\n   Subject: %s\n   Date: %s\n   ID: %s\n\n",
			i+1, from, subject, date, m.Id))
	}
	msg := sb.String()
	return &ToolResult{ForLLM: msg, ForUser: msg}
}

func (t *GmailTool) read(ctx context.Context, args map[string]any) *ToolResult {
	svc, err := t.service()
	if err != nil {
		return ErrorResult(err.Error())
	}

	msgID, _ := args["message_id"].(string)
	if msgID == "" {
		return ErrorResult("gmail read: 'message_id' is required")
	}

	full, err := svc.Users.Messages.Get("me", msgID).Format("full").Context(ctx).Do()
	if err != nil {
		return ErrorResult(fmt.Sprintf("gmail read: %v", err))
	}

	subject, from, date := extractHeaders(full)
	body := extractBody(full)

	msg := fmt.Sprintf("📧 From: %s\nDate: %s\nSubject: %s\n\n%s", from, date, subject, body)
	return &ToolResult{ForLLM: msg, ForUser: msg}
}

func (t *GmailTool) search(ctx context.Context, args map[string]any) *ToolResult {
	svc, err := t.service()
	if err != nil {
		return ErrorResult(err.Error())
	}

	query, _ := args["query"].(string)
	if query == "" {
		return ErrorResult("gmail search: 'query' is required")
	}

	count := int64(10)
	if c, ok := args["count"].(float64); ok && c > 0 {
		count = int64(c)
	}

	resp, err := svc.Users.Messages.List("me").
		Q(query).
		MaxResults(count).
		Context(ctx).Do()
	if err != nil {
		return ErrorResult(fmt.Sprintf("gmail search: %v", err))
	}

	if len(resp.Messages) == 0 {
		msg := fmt.Sprintf("No emails found for query: %q", query)
		return &ToolResult{ForLLM: msg, ForUser: msg}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 Search results for %q (%d found):\n\n", query, len(resp.Messages)))
	for i, m := range resp.Messages {
		full, err := svc.Users.Messages.Get("me", m.Id).
			Format("metadata").
			MetadataHeaders("Subject", "From", "Date").
			Context(ctx).Do()
		if err != nil {
			continue
		}
		subject, from, date := extractHeaders(full)
		sb.WriteString(fmt.Sprintf("%d. From: %s\n   Subject: %s\n   Date: %s\n   ID: %s\n\n",
			i+1, from, subject, date, m.Id))
	}
	msg := sb.String()
	return &ToolResult{ForLLM: msg, ForUser: msg}
}

func (t *GmailTool) schedule(ctx context.Context, args map[string]any) *ToolResult {
	if t.scheduler == nil {
		return ErrorResult("gmail schedule: email scheduler not configured")
	}
	toStr, _ := args["to"].(string)
	subject, _ := args["subject"].(string)
	body, _ := args["body"].(string)
	if toStr == "" || subject == "" || body == "" {
		return ErrorResult("gmail schedule: 'to', 'subject', and 'body' are required")
	}
	if err := t.scheduler.Enqueue(ctx, toStr, subject, body); err != nil {
		return ErrorResult(fmt.Sprintf("gmail schedule: %v", err))
	}
	llmMsg := fmt.Sprintf("✅ Email queued to %s — Subject: %q. The dispatcher will send it automatically within 30 seconds. Do NOT call 'send' action — it will be sent automatically.", toStr, subject)
	userMsg := fmt.Sprintf("✅ Email scheduled to %s — Subject: %q (will be sent within 30 seconds)", toStr, subject)
	return &ToolResult{ForLLM: llmMsg, ForUser: userMsg}
}

func (t *GmailTool) listScheduled(ctx context.Context) *ToolResult {
	if t.scheduler == nil {
		return ErrorResult("gmail list_scheduled: email scheduler not configured")
	}
	count, err := t.scheduler.GetPendingCount(ctx)
	if err != nil {
		return ErrorResult(fmt.Sprintf("gmail list_scheduled: %v", err))
	}
	if count == 0 {
		msg := "No scheduled emails."
		return &ToolResult{ForLLM: msg, ForUser: msg}
	}
	msg := fmt.Sprintf("📋 %d scheduled email(s) pending.", count)
	return &ToolResult{ForLLM: msg, ForUser: msg}
}

func (t *GmailTool) cancelScheduled(ctx context.Context, args map[string]any) *ToolResult {
	if t.scheduler == nil {
		return ErrorResult("gmail cancel_scheduled: email scheduler not configured")
	}
	msg := "✅ Cancel not supported in this version. Use list_scheduled to see pending emails."
	return &ToolResult{ForLLM: msg, ForUser: msg}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func extractHeaders(msg *gmail.Message) (subject, from, date string) {
	for _, h := range msg.Payload.Headers {
		switch h.Name {
		case "Subject":
			subject = h.Value
		case "From":
			from = h.Value
		case "Date":
			date = h.Value
		}
	}
	return
}

func extractBody(msg *gmail.Message) string {
	if msg.Payload == nil {
		return ""
	}
	// Try direct body first
	if msg.Payload.Body != nil && msg.Payload.Body.Data != "" {
		decoded, err := base64.URLEncoding.DecodeString(msg.Payload.Body.Data)
		if err == nil {
			return string(decoded)
		}
	}
	// Try parts
	for _, part := range msg.Payload.Parts {
		if part.MimeType == "text/plain" && part.Body != nil && part.Body.Data != "" {
			decoded, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err == nil {
				return string(decoded)
			}
		}
	}
	return "(no body)"
}
