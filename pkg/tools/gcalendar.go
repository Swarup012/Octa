// Octa - Personal AI Agent
// License: MIT

package tools

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// GoogleCalendarTool implements the Tool interface for Google Calendar.
// The *calendar.Service is created lazily on first use via sync.Once.
type GoogleCalendarTool struct {
	httpClient   *http.Client
	UserTimezone string

	once sync.Once
	svc  *calendar.Service
}

// NewGoogleCalendarTool creates a GoogleCalendarTool. Service is lazy.
func NewGoogleCalendarTool(httpClient *http.Client) *GoogleCalendarTool {
	return &GoogleCalendarTool{
		httpClient:   httpClient,
		UserTimezone: "Asia/Kolkata",
	}
}

func (t *GoogleCalendarTool) Name() string { return "google_calendar" }

func (t *GoogleCalendarTool) Description() string {
	return "Manage Google Calendar events. Actions: create, list, find, update, delete, delete_range."
}

func (t *GoogleCalendarTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action":     map[string]any{"type": "string", "enum": []string{"create", "list", "find", "update", "delete", "delete_range"}},
			"summary":    map[string]any{"type": "string", "description": "Event title"},
			"start_time": map[string]any{"type": "string", "description": "Start time RFC3339 or 'YYYY-MM-DD HH:MM'"},
			"end_time":   map[string]any{"type": "string", "description": "End time RFC3339 or 'YYYY-MM-DD HH:MM'"},
			"event_id":   map[string]any{"type": "string", "description": "Event ID for update/delete"},
			"query":      map[string]any{"type": "string", "description": "Search query for find action"},
			"add_meet":   map[string]any{"type": "boolean", "description": "Add Google Meet (default true)"},
			"attendees":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "List of attendee emails"},
			"count":      map[string]any{"type": "integer", "description": "Number of events to return (default 10)"},
		},
		"required": []string{"action"},
	}
}

// service returns the cached *calendar.Service, lazy-initialised via sync.Once.
func (t *GoogleCalendarTool) service() (*calendar.Service, error) {
	t.once.Do(func() {
		svc, err := calendar.NewService(context.Background(), option.WithHTTPClient(t.httpClient))
		if err != nil {
			return
		}
		t.svc = svc
	})
	if t.svc == nil {
		return nil, fmt.Errorf("google_calendar: service failed to initialise — check OAuth token")
	}
	return t.svc, nil
}

// ListUpcoming fetches upcoming events within the given window. Used by the dispatcher.
func (t *GoogleCalendarTool) ListUpcoming(ctx context.Context, window time.Duration) ([]*calendar.Event, error) {
	svc, err := t.service()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	resp, err := svc.Events.List("primary").
		TimeMin(now.Format(time.RFC3339)).
		TimeMax(now.Add(window).Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		MaxResults(50).
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (t *GoogleCalendarTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	svc, err := t.service()
	if err != nil {
		return ErrorResult(err.Error())
	}

	action, _ := args["action"].(string)
	switch action {
	case "create":
		return t.create(ctx, svc, args)
	case "list":
		return t.list(ctx, svc, args)
	case "find":
		return t.find(ctx, svc, args)
	case "update":
		return t.update(ctx, svc, args)
	case "delete":
		return t.deleteEvent(ctx, svc, args)
	case "delete_range":
		return t.deleteRange(ctx, svc, args)
	default:
		return ErrorResult(fmt.Sprintf("google_calendar: unknown action %q", action))
	}
}

func (t *GoogleCalendarTool) parseTime(s string) (string, error) {
	if s == "" {
		return "", fmt.Errorf("empty time string")
	}
	// Try RFC3339 first
	if _, err := time.Parse(time.RFC3339, s); err == nil {
		return s, nil
	}
	// Try YYYY-MM-DD HH:MM in user timezone
	loc, err := time.LoadLocation(t.UserTimezone)
	if err != nil {
		loc = time.UTC
	}
	formats := []string{"2006-01-02 15:04", "2006-01-02T15:04", "2006-01-02 15:04:05"}
	for _, f := range formats {
		if parsed, err := time.ParseInLocation(f, s, loc); err == nil {
			return parsed.Format(time.RFC3339), nil
		}
	}
	return "", fmt.Errorf("cannot parse time %q", s)
}

func (t *GoogleCalendarTool) create(ctx context.Context, svc *calendar.Service, args map[string]any) *ToolResult {
	summary, _ := args["summary"].(string)
	if summary == "" {
		return ErrorResult("google_calendar create: 'summary' is required")
	}

	startStr, _ := args["start_time"].(string)
	endStr, _ := args["end_time"].(string)

	startRFC, err := t.parseTime(startStr)
	if err != nil {
		return ErrorResult(fmt.Sprintf("google_calendar create: invalid start_time: %v", err))
	}
	if endStr == "" {
		// Default: 1 hour after start
		st, _ := time.Parse(time.RFC3339, startRFC)
		endStr = st.Add(time.Hour).Format(time.RFC3339)
	}
	endRFC, err := t.parseTime(endStr)
	if err != nil {
		return ErrorResult(fmt.Sprintf("google_calendar create: invalid end_time: %v", err))
	}

	event := &calendar.Event{
		Summary: summary,
		Start:   &calendar.EventDateTime{DateTime: startRFC},
		End:     &calendar.EventDateTime{DateTime: endRFC},
	}

	if desc, ok := args["description"].(string); ok && desc != "" {
		event.Description = desc
	}

	// Add Google Meet by default
	addMeet := true
	if v, ok := args["add_meet"].(bool); ok {
		addMeet = v
	}
	if addMeet {
		event.ConferenceData = &calendar.ConferenceData{
			CreateRequest: &calendar.CreateConferenceRequest{
				RequestId:             fmt.Sprintf("meet-%d", time.Now().UnixNano()),
				ConferenceSolutionKey: &calendar.ConferenceSolutionKey{Type: "hangoutsMeet"},
			},
		}
	}

	// Attendees
	if rawAttendees, ok := args["attendees"]; ok {
		switch v := rawAttendees.(type) {
		case []any:
			for _, a := range v {
				if email, ok := a.(string); ok && email != "" {
					event.Attendees = append(event.Attendees, &calendar.EventAttendee{Email: email})
				}
			}
		}
	}

	confDataVersion := int64(0)
	if addMeet {
		confDataVersion = 1
	}

	created, err := svc.Events.Insert("primary", event).
		ConferenceDataVersion(confDataVersion).
		Context(ctx).Do()
	if err != nil {
		return ErrorResult(fmt.Sprintf("google_calendar create: %v", err))
	}

	meetLink := ""
	if created.ConferenceData != nil && len(created.ConferenceData.EntryPoints) > 0 {
		meetLink = "\n  Meet: " + created.ConferenceData.EntryPoints[0].Uri
	}
	msg := fmt.Sprintf("✅ Event created: %q\n  ID: %s\n  Start: %s\n  End: %s%s",
		created.Summary, created.Id, created.Start.DateTime, created.End.DateTime, meetLink)
	return &ToolResult{ForLLM: msg, ForUser: msg}
}

func (t *GoogleCalendarTool) list(ctx context.Context, svc *calendar.Service, args map[string]any) *ToolResult {
	count := int64(10)
	if c, ok := args["count"].(float64); ok && c > 0 {
		count = int64(c)
	}

	now := time.Now()
	resp, err := svc.Events.List("primary").
		TimeMin(now.Format(time.RFC3339)).
		TimeMax(now.Add(7 * 24 * time.Hour).Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		MaxResults(count).
		Context(ctx).Do()
	if err != nil {
		return ErrorResult(fmt.Sprintf("google_calendar list: %v", err))
	}

	if len(resp.Items) == 0 {
		msg := "No upcoming events in the next 7 days."
		return &ToolResult{ForLLM: msg, ForUser: msg}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📅 Upcoming events (%d):\n\n", len(resp.Items)))
	for i, e := range resp.Items {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, formatCalEvent(e)))
	}
	msg := sb.String()
	return &ToolResult{ForLLM: msg, ForUser: msg}
}

func (t *GoogleCalendarTool) find(ctx context.Context, svc *calendar.Service, args map[string]any) *ToolResult {
	query, _ := args["query"].(string)
	if query == "" {
		return ErrorResult("google_calendar find: 'query' is required")
	}

	resp, err := svc.Events.List("primary").
		Q(query).
		SingleEvents(true).
		OrderBy("startTime").
		MaxResults(10).
		Context(ctx).Do()
	if err != nil {
		return ErrorResult(fmt.Sprintf("google_calendar find: %v", err))
	}

	if len(resp.Items) == 0 {
		msg := fmt.Sprintf("No events found matching %q.", query)
		return &ToolResult{ForLLM: msg, ForUser: msg}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 Events matching %q (%d):\n\n", query, len(resp.Items)))
	for i, e := range resp.Items {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, formatCalEvent(e)))
	}
	msg := sb.String()
	return &ToolResult{ForLLM: msg, ForUser: msg}
}

func (t *GoogleCalendarTool) update(ctx context.Context, svc *calendar.Service, args map[string]any) *ToolResult {
	eventID, _ := args["event_id"].(string)
	if eventID == "" {
		return ErrorResult("google_calendar update: 'event_id' is required")
	}

	existing, err := svc.Events.Get("primary", eventID).Context(ctx).Do()
	if err != nil {
		return ErrorResult(fmt.Sprintf("google_calendar update: event not found — %v", err))
	}

	if summary, ok := args["summary"].(string); ok && summary != "" {
		existing.Summary = summary
	}
	if desc, ok := args["description"].(string); ok && desc != "" {
		existing.Description = desc
	}
	if startStr, ok := args["start_time"].(string); ok && startStr != "" {
		startRFC, err := t.parseTime(startStr)
		if err != nil {
			return ErrorResult(fmt.Sprintf("google_calendar update: invalid start_time: %v", err))
		}
		existing.Start = &calendar.EventDateTime{DateTime: startRFC}
	}
	if endStr, ok := args["end_time"].(string); ok && endStr != "" {
		endRFC, err := t.parseTime(endStr)
		if err != nil {
			return ErrorResult(fmt.Sprintf("google_calendar update: invalid end_time: %v", err))
		}
		existing.End = &calendar.EventDateTime{DateTime: endRFC}
	}

	updated, err := svc.Events.Update("primary", eventID, existing).Context(ctx).Do()
	if err != nil {
		return ErrorResult(fmt.Sprintf("google_calendar update: %v", err))
	}

	msg := fmt.Sprintf("✅ Event updated: %q\n  Start: %s\n  End: %s",
		updated.Summary, updated.Start.DateTime, updated.End.DateTime)
	return &ToolResult{ForLLM: msg, ForUser: msg}
}

func (t *GoogleCalendarTool) deleteEvent(ctx context.Context, svc *calendar.Service, args map[string]any) *ToolResult {
	eventID, _ := args["event_id"].(string)
	if eventID == "" {
		return ErrorResult("google_calendar delete: 'event_id' is required")
	}

	err := svc.Events.Delete("primary", eventID).Context(ctx).Do()
	if err != nil {
		return ErrorResult(fmt.Sprintf("google_calendar delete: %v", err))
	}

	msg := fmt.Sprintf("🗑️ Event deleted (ID: %s)", eventID)
	return &ToolResult{ForLLM: msg, ForUser: msg}
}

func (t *GoogleCalendarTool) deleteRange(ctx context.Context, svc *calendar.Service, args map[string]any) *ToolResult {
	startStr, _ := args["start_time"].(string)
	endStr, _ := args["end_time"].(string)
	if startStr == "" || endStr == "" {
		return ErrorResult("google_calendar delete_range: 'start_time' and 'end_time' are required")
	}

	startRFC, err := t.parseTime(startStr)
	if err != nil {
		return ErrorResult(fmt.Sprintf("google_calendar delete_range: invalid start_time: %v", err))
	}
	endRFC, err := t.parseTime(endStr)
	if err != nil {
		return ErrorResult(fmt.Sprintf("google_calendar delete_range: invalid end_time: %v", err))
	}

	resp, err := svc.Events.List("primary").
		TimeMin(startRFC).
		TimeMax(endRFC).
		SingleEvents(true).
		Context(ctx).Do()
	if err != nil {
		return ErrorResult(fmt.Sprintf("google_calendar delete_range: fetch events — %v", err))
	}

	deleted := 0
	for _, e := range resp.Items {
		if delErr := svc.Events.Delete("primary", e.Id).Context(ctx).Do(); delErr == nil {
			deleted++
		}
	}

	msg := fmt.Sprintf("🗑️ Deleted %d event(s) between %s and %s.", deleted, startStr, endStr)
	return &ToolResult{ForLLM: msg, ForUser: msg}
}

func formatCalEvent(e *calendar.Event) string {
	start := e.Start.DateTime
	if start == "" {
		start = e.Start.Date
	}
	end := e.End.DateTime
	if end == "" {
		end = e.End.Date
	}
	return fmt.Sprintf("%s\n  ID: %s\n  Start: %s\n  End: %s", e.Summary, e.Id, start, end)
}
