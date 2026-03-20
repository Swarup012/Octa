package tools

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// EditFileTool edits a file by replacing old_text with new_text.
// The old_text must exist exactly in the file.
type EditFileTool struct {
	channel  string
	chatID   string
	workspace string
	restrict  bool
}

// NewEditFileTool creates a new EditFileTool with optional directory restriction.
func NewEditFileTool(workspace string, restrict bool) *EditFileTool {
	return &EditFileTool{
		workspace: workspace,
		restrict:  restrict,
	}
}

// SetContext sets the channel and chatID for context-aware operations.
func (t *EditFileTool) SetContext(channel, chatID string) {
	t.channel = channel
	t.chatID = chatID
}

func (t *EditFileTool) Name() string {
	return "edit_file"
}

func (t *EditFileTool) Description() string {
	return "Edit a file by replacing old_text with new_text. The old_text must exist exactly in the file."
}

func (t *EditFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The file path to edit",
			},
			"old_text": map[string]any{
				"type":        "string",
				"description": "The exact text to find and replace",
			},
			"new_text": map[string]any{
				"type":        "string",
				"description": "The text to replace with",
			},
		},
		"required": []string{"path", "old_text", "new_text"},
	}
}

func (t *EditFileTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	path, ok := args["path"].(string)
	if !ok {
		return ErrorResult("path is required")
	}

	oldText, ok := args["old_text"].(string)
	if !ok {
		return ErrorResult("old_text is required")
	}

	newText, ok := args["new_text"].(string)
	if !ok {
		return ErrorResult("new_text is required")
	}

	// Check danger zones first (always blocked)
	if isDangerZone(path) {
		return ErrorResult("Access denied: " + path + " is protected")
	}

	// Check if path is outside workspace when restricted
	if t.restrict {
		pathAbs, err := filepath.Abs(path)
		if err == nil {
			workspaceAbs, _ := filepath.Abs(t.workspace)
			rel, err := filepath.Rel(workspaceAbs, pathAbs)
			if err == nil && strings.HasPrefix(rel, "..") {
				// Path is outside workspace - check for permission
				hasSudo, accessLevel := HasExecSudo(t.channel, t.chatID)
				if !hasSudo || accessLevel != AccessUnrestricted {
					// No permission - ask for approval with formatted prompt
					prompt := BuildFileOperationPermissionPrompt("Edit file", path)
					return &ToolResult{
						ForLLM:  prompt + "\n\nDo NOT retry the operation until the user grants approval.",
						ForUser: prompt,
						IsError: false,
					}
				}
			}
		}
	}

	// Determine which filesystem to use based on current permission
	var fs fileSystem
	if t.restrict {
		hasSudo, accessLevel := HasExecSudo(t.channel, t.chatID)
		if hasSudo && accessLevel == AccessUnrestricted {
			// Use hostFs (unrestricted)
			fs = &hostFs{}
		} else {
			// Use sandboxFs (restricted to workspace)
			fs = &sandboxFs{workspace: t.workspace}
		}
	} else {
		// No restriction configured, always use hostFs
		fs = &hostFs{}
	}

	if err := editFile(fs, path, oldText, newText); err != nil {
		return ErrorResult(err.Error())
	}
	return SilentResult(fmt.Sprintf("File edited: %s", path))
}

type AppendFileTool struct {
	channel  string
	chatID   string
	workspace string
	restrict  bool
}

func NewAppendFileTool(workspace string, restrict bool) *AppendFileTool {
	return &AppendFileTool{
		workspace: workspace,
		restrict:  restrict,
	}
}

func (t *AppendFileTool) SetContext(channel, chatID string) {
	t.channel = channel
	t.chatID = chatID
}

func (t *AppendFileTool) Name() string {
	return "append_file"
}

func (t *AppendFileTool) Description() string {
	return "Append content to the end of a file"
}

func (t *AppendFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The file path to append to",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The content to append",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (t *AppendFileTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	path, ok := args["path"].(string)
	if !ok {
		return ErrorResult("path is required")
	}

	content, ok := args["content"].(string)
	if !ok {
		return ErrorResult("content is required")
	}

	// Check danger zones first (always blocked)
	if isDangerZone(path) {
		return ErrorResult("Access denied: " + path + " is protected")
	}

	// Check if path is outside workspace when restricted
	if t.restrict {
		pathAbs, err := filepath.Abs(path)
		if err == nil {
			workspaceAbs, _ := filepath.Abs(t.workspace)
			rel, err := filepath.Rel(workspaceAbs, pathAbs)
			if err == nil && strings.HasPrefix(rel, "..") {
				// Path is outside workspace - check for permission
				hasSudo, accessLevel := HasExecSudo(t.channel, t.chatID)
				if !hasSudo || accessLevel != AccessUnrestricted {
					// No permission - ask for approval with formatted prompt for AppendFile
					prompt := BuildFileOperationPermissionPrompt("Append to file", path)
					return &ToolResult{
						ForLLM:  prompt + "\n\nDo NOT retry the operation until the user grants approval.",
						ForUser: prompt,
						IsError: false,
					}
				}
			}
		}
	}

	// Determine which filesystem to use based on current permission
	var fs fileSystem
	if t.restrict {
		hasSudo, accessLevel := HasExecSudo(t.channel, t.chatID)
		if hasSudo && accessLevel == AccessUnrestricted {
			// Use hostFs (unrestricted)
			fs = &hostFs{}
		} else {
			// Use sandboxFs (restricted to workspace)
			fs = &sandboxFs{workspace: t.workspace}
		}
	} else {
		// No restriction configured, always use hostFs
		fs = &hostFs{}
	}

	if err := appendFile(fs, path, content); err != nil {
		return ErrorResult(err.Error())
	}
	return SilentResult(fmt.Sprintf("Appended to %s", path))
}

// editFile reads the file via sysFs, performs the replacement, and writes back.
// It uses a fileSystem interface, allowing the same logic for both restricted and unrestricted modes.
func editFile(sysFs fileSystem, path, oldText, newText string) error {
	content, err := sysFs.ReadFile(path)
	if err != nil {
		return err
	}

	newContent, err := replaceEditContent(content, oldText, newText)
	if err != nil {
		return err
	}

	return sysFs.WriteFile(path, newContent)
}

// appendFile reads the existing content (if any) via sysFs, appends new content, and writes back.
func appendFile(sysFs fileSystem, path, appendContent string) error {
	content, err := sysFs.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	newContent := append(content, []byte(appendContent)...)
	return sysFs.WriteFile(path, newContent)
}

// replaceEditContent handles the core logic of finding and replacing a single occurrence of oldText.
func replaceEditContent(content []byte, oldText, newText string) ([]byte, error) {
	contentStr := string(content)

	if !strings.Contains(contentStr, oldText) {
		return nil, fmt.Errorf("old_text not found in file. Make sure it matches exactly")
	}

	count := strings.Count(contentStr, oldText)
	if count > 1 {
		return nil, fmt.Errorf("old_text appears %d times. Please provide more context to make it unique", count)
	}

	newContent := strings.Replace(contentStr, oldText, newText, 1)
	return []byte(newContent), nil
}
