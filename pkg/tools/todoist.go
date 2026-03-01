package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"

)

const todoistBaseURL = "https://api.todoist.com/api/v1"

type todoistDue struct {
	String    string `json:"string,omitempty"`
	Date      string `json:"date,omitempty"`
	IsRecurring bool `json:"is_recurring"`
}

type todoistTask struct {
	ID          string         `json:"id"`
	Content     string         `json:"content"`
	Description string         `json:"description,omitempty"`
	ProjectID   string         `json:"project_id"`
	Priority    int            `json:"priority"`
	Due         *todoistDue    `json:"due,omitempty"`
	Checked     bool           `json:"checked"`
	Labels      []string       `json:"labels,omitempty"`
	AddedAt     string         `json:"added_at"`
}

type todoistProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type todoistPagedTasks struct {
	Results []todoistTask `json:"tasks"`
}

type todoistPagedProjects struct {
	Results []todoistProject `json:"projects"`
}

type TodoistTool struct {
	apiToken   string
	httpClient *http.Client
	initOnce   sync.Once
}

func NewTodoistTool(apiToken string) *TodoistTool {
	token := apiToken
	return &TodoistTool{
		apiToken: token,
	}
}

func (t *TodoistTool) Name() string {
	return "todoist"
}

func (t *TodoistTool) Description() string {
	return "Manage Todoist tasks and projects. Actions: create, create_bulk, list, complete, complete_all, delete, delete_all, update"
}

func (t *TodoistTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Action: create, create_bulk, list, complete, complete_all, delete, delete_all, update",
				"enum": []string{"create", "create_bulk", "list", "complete", "complete_all", "delete", "delete_all", "update"},
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Task content/title",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Task description",
			},
			"project_id": map[string]any{
				"type":        "string",
				"description": "Project ID",
			},
			"priority": map[string]any{
				"type":        "integer",
				"description": "Priority: 4=Urgent(p1), 3=High(p2), 2=Medium(p3), 1=Normal",
			},
			"due_string": map[string]any{
				"type":        "string",
				"description": "Due date (natural language or ISO format)",
			},
			"labels": map[string]any{
				"type":        "array",
				"description": "Task labels",
				"items": map[string]any{
					"type": "string",
				},
			},
			"task_id": map[string]any{
				"type":        "string",
				"description": "Task ID for complete, delete, update actions",
			},
			"tasks": map[string]any{
				"type":        "array",
				"description": "Array of tasks for create_bulk (each task object with content, project_id, priority, due_string, labels, description)",
				"items": map[string]any{
					"type": "object",
				},
			},
			"filter": map[string]any{
				"type":        "string",
				"description": "Filter string for complete_all/delete_all (e.g., 'today', 'overdue')",
			},
		},
		"required": []string{"action"},
	}
}

func (t *TodoistTool) getHTTPClient() *http.Client {
	t.initOnce.Do(func() {
		t.httpClient = &http.Client{}
	})
	return t.httpClient
}

func encodeQueryParam(s string) string {
	return url.QueryEscape(s)
}

func (t *TodoistTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, _ := args["action"].(string)
	// Backward-compatible aliases
	if action == "add" {
		action = "create"
	}
	if action == "create" {
		if _, ok := args["content"]; !ok {
			if item, ok := args["item"]; ok {
				args["content"] = item
			}
		}
	}

	switch action {
	case "create":
		return t.actionCreate(ctx, args)
	case "create_bulk":
		return t.actionCreateBulk(ctx, args)
	case "list":
		return t.actionList(ctx, args)
	case "complete":
		return t.actionComplete(ctx, args)
	case "complete_all":
		return t.actionCompleteAll(ctx, args)
	case "delete":
		return t.actionDelete(ctx, args)
	case "delete_all":
		return t.actionDeleteAll(ctx, args)
	case "update":
		return t.actionUpdate(ctx, args)
	default:
		return ErrorResult(fmt.Sprintf("Unknown action: %s", action))
	}
}

func (t *TodoistTool) actionCreate(ctx context.Context, args map[string]any) *ToolResult {
	content, ok := args["content"].(string)
	if !ok {
		return ErrorResult("content is required")
	}

	body := map[string]any{
		"content": content,
	}

	if projectID, ok := args["project_id"].(string); ok && projectID != "" {
		body["project_id"] = projectID
	}
	if priority, ok := args["priority"].(float64); ok {
		body["priority"] = int(priority)
	}
	if description, ok := args["description"].(string); ok && description != "" {
		body["description"] = description
	}
	if dueString, ok := args["due_string"].(string); ok && dueString != "" {
		body["due_string"] = dueString
	}
	if labels, ok := args["labels"].([]any); ok && len(labels) > 0 {
		body["labels"] = labels
	}

	resp, err := t.doRequest(ctx, "POST", "/tasks", body)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to create task: %v", err)).WithError(err)
	}

	var task todoistTask
	if err := json.Unmarshal(resp, &task); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to parse response: %v", err)).WithError(err)
	}

	return UserResult(fmt.Sprintf("Task created: %s (ID: %s)", task.Content, task.ID))
}

func (t *TodoistTool) actionCreateBulk(ctx context.Context, args map[string]any) *ToolResult {
	tasksRaw, ok := args["tasks"]
	if !ok {
		return ErrorResult("tasks array is required")
	}

	var tasks []map[string]any

	switch v := tasksRaw.(type) {
	case string:
		// LLM sent it as a JSON string
		if err := json.Unmarshal([]byte(v), &tasks); err != nil {
			return ErrorResult(fmt.Sprintf("Failed to parse tasks JSON: %v", err)).WithError(err)
		}
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				tasks = append(tasks, m)
			}
		}
	default:
		return ErrorResult("tasks must be an array or JSON string")
	}

	if len(tasks) == 0 {
		return ErrorResult("tasks array is empty")
	}

	var created []string
	var errs []string

	for _, taskData := range tasks {
		content, ok := taskData["content"].(string)
		if !ok {
			errs = append(errs, "skipped task: missing content")
			continue
		}

		body := map[string]any{"content": content}
		if projectID, ok := taskData["project_id"].(string); ok && projectID != "" {
			body["project_id"] = projectID
		}
		if priority, ok := taskData["priority"].(float64); ok {
			body["priority"] = int(priority)
		}
		if description, ok := taskData["description"].(string); ok && description != "" {
			body["description"] = description
		}
		if dueString, ok := taskData["due_string"].(string); ok && dueString != "" {
			body["due_string"] = dueString
		}
		if labels, ok := taskData["labels"].([]any); ok && len(labels) > 0 {
			body["labels"] = labels
		}

		resp, err := t.doRequest(ctx, "POST", "/tasks", body)
		if err != nil {
			errs = append(errs, fmt.Sprintf("failed to create '%s': %v", content, err))
			continue
		}

		var task todoistTask
		if err := json.Unmarshal(resp, &task); err != nil {
			errs = append(errs, fmt.Sprintf("failed to parse task '%s': %v", content, err))
			continue
		}

		created = append(created, task.ID)
	}

	msg := fmt.Sprintf("Created %d tasks", len(created))
	if len(errs) > 0 {
		msg += fmt.Sprintf(" with %d errors", len(errs))
	}

	return UserResult(msg)
}

func (t *TodoistTool) actionList(ctx context.Context, args map[string]any) *ToolResult {
	filter, _ := args["filter"].(string)

	endpoint := "/tasks"
	if filter != "" {
		endpoint += "?filter=" + encodeQueryParam(filter)
	}

	resp, err := t.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to list tasks: %v", err)).WithError(err)
	}

	var paged todoistPagedTasks
	if err := json.Unmarshal(resp, &paged); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to parse response: %v", err)).WithError(err)
	}

	if len(paged.Results) == 0 {
		return UserResult("No tasks found")
	}

	var result string
	for _, task := range paged.Results {
		status := "☐"
		if task.Checked {
			status = "☑"
		}
		result += fmt.Sprintf("%s %s (ID: %s, Priority: %d)\n", status, task.Content, task.ID, task.Priority)
	}

	return UserResult(result)
}

func (t *TodoistTool) actionComplete(ctx context.Context, args map[string]any) *ToolResult {
	taskID, ok := args["task_id"].(string)
	if !ok {
		return ErrorResult("task_id is required")
	}

	_, err := t.doRequest(ctx, "POST", fmt.Sprintf("/tasks/%s/close", taskID), nil)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to complete task: %v", err)).WithError(err)
	}

	return UserResult(fmt.Sprintf("Task %s completed", taskID))
}

func (t *TodoistTool) actionCompleteAll(ctx context.Context, args map[string]any) *ToolResult {
	filter, ok := args["filter"].(string)
	if !ok || filter == "" {
		return ErrorResult("filter is required for complete_all")
	}

	// Fetch matching tasks
	endpoint := "/tasks?filter=" + encodeQueryParam(filter)
	resp, err := t.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to fetch tasks: %v", err)).WithError(err)
	}

	var paged todoistPagedTasks
	if err := json.Unmarshal(resp, &paged); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to parse response: %v", err)).WithError(err)
	}

	if len(paged.Results) == 0 {
		return UserResult("No tasks matched the filter")
	}

	var completed int
	var failed int

	for _, task := range paged.Results {
		_, err := t.doRequest(ctx, "POST", fmt.Sprintf("/tasks/%s/close", task.ID), nil)
		if err != nil {
			failed++
		} else {
			completed++
		}
	}

	return UserResult(fmt.Sprintf("Completed %d tasks (failed: %d)", completed, failed))
}

func (t *TodoistTool) actionDelete(ctx context.Context, args map[string]any) *ToolResult {
	taskID, ok := args["task_id"].(string)
	if !ok {
		return ErrorResult("task_id is required")
	}

	_, err := t.doRequest(ctx, "DELETE", fmt.Sprintf("/tasks/%s", taskID), nil)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to delete task: %v", err)).WithError(err)
	}

	return UserResult(fmt.Sprintf("Task %s deleted", taskID))
}

func (t *TodoistTool) actionDeleteAll(ctx context.Context, args map[string]any) *ToolResult {
	filter, ok := args["filter"].(string)
	if !ok || filter == "" {
		return ErrorResult("filter is required for delete_all")
	}

	// Fetch matching tasks
	endpoint := "/tasks?filter=" + encodeQueryParam(filter)
	resp, err := t.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to fetch tasks: %v", err)).WithError(err)
	}

	var paged todoistPagedTasks
	if err := json.Unmarshal(resp, &paged); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to parse response: %v", err)).WithError(err)
	}

	if len(paged.Results) == 0 {
		return UserResult("No tasks matched the filter")
	}

	var deleted int
	var failed int

	for _, task := range paged.Results {
		_, err := t.doRequest(ctx, "DELETE", fmt.Sprintf("/tasks/%s", task.ID), nil)
		if err != nil {
			failed++
		} else {
			deleted++
		}
	}

	return UserResult(fmt.Sprintf("Deleted %d tasks (failed: %d)", deleted, failed))
}

func (t *TodoistTool) actionUpdate(ctx context.Context, args map[string]any) *ToolResult {
	taskID, ok := args["task_id"].(string)
	if !ok {
		return ErrorResult("task_id is required")
	}

	body := make(map[string]any)

	if content, ok := args["content"].(string); ok && content != "" {
		body["content"] = content
	}
	if description, ok := args["description"].(string); ok && description != "" {
		body["description"] = description
	}
	if priority, ok := args["priority"].(float64); ok {
		body["priority"] = int(priority)
	}
	if dueString, ok := args["due_string"].(string); ok && dueString != "" {
		body["due_string"] = dueString
	}
	if labels, ok := args["labels"].([]any); ok && len(labels) > 0 {
		body["labels"] = labels
	}

	if len(body) == 0 {
		return ErrorResult("At least one field to update is required")
	}

	_, err := t.doRequest(ctx, "POST", fmt.Sprintf("/tasks/%s", taskID), body)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to update task: %v", err)).WithError(err)
	}

	return UserResult(fmt.Sprintf("Task %s updated", taskID))
}

func (t *TodoistTool) doRequest(ctx context.Context, method, endpoint string, body map[string]any) ([]byte, error) {
	if t.apiToken == "" {
		return nil, fmt.Errorf("API token not configured")
	}

	client := t.getHTTPClient()
	url := todoistBaseURL + endpoint
	var req *http.Request
	var err error

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(jsonBody))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}

	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+t.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
