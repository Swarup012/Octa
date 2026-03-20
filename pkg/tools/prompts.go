package tools

import "strings"

// PermissionRequest represents a permission request from the agent
type PermissionRequest struct {
	Operation string
	Target    string
	Reason    string
}

// BuildPermissionPrompt creates a nicely formatted permission prompt message
// that works across all channels (CLI, Telegram, Discord, WhatsApp)
func BuildPermissionPrompt(request PermissionRequest) string {
	var builder strings.Builder

	// Header with emoji for attention
	builder.WriteString("⚠️ Permission required\n\n")

	// What the agent is trying to do
	builder.WriteString("Operation: ")
	builder.WriteString(request.Operation)
	builder.WriteString("\n")

	// Target (file, path, command, etc.)
	if request.Target != "" {
		builder.WriteString("Target: ")
		builder.WriteString(request.Target)
		builder.WriteString("\n")
	}

	// Additional reason if provided
	if request.Reason != "" {
		builder.WriteString("Reason: ")
		builder.WriteString(request.Reason)
		builder.WriteString("\n")
	}

	builder.WriteString("\n")
	builder.WriteString("Quick approve:\n")
	builder.WriteString("  - /approve (one-time)\n")
	builder.WriteString("  - /approve 5m (5 minutes)\n")
	builder.WriteString("  - Ignore to keep restrictions enabled")

	return builder.String()
}

// BuildShellPermissionPrompt creates a permission prompt for shell commands
func BuildShellPermissionPrompt(command string) string {
	truncate := 100
	if len(command) > truncate {
		command = command[:truncate] + "..."
	}

	return BuildPermissionPrompt(PermissionRequest{
		Operation: "Execute shell command",
		Target:    command,
	})
}

// BuildFileOperationPermissionPrompt creates a permission prompt for file operations
func BuildFileOperationPermissionPrompt(operation, path string) string {
	return BuildPermissionPrompt(PermissionRequest{
		Operation: operation,
		Target:    path,
		Reason:    "Outside workspace directory",
	})
}

// BuildShellExecutionDenied creates a message for when shell execution is denied
func BuildShellExecutionDenied() string {
	return BuildPermissionPrompt(PermissionRequest{
		Operation: "Execute shell command",
		Reason:    "No permission granted",
	})
}