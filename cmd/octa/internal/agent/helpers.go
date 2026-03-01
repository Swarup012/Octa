package agent

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chzyer/readline"

	"github.com/Swarup012/solo/cmd/octa/internal"
	"github.com/Swarup012/solo/pkg/agent"
	"github.com/Swarup012/solo/pkg/bus"
	"github.com/Swarup012/solo/pkg/cron"
	"github.com/Swarup012/solo/pkg/integrations"
	"github.com/Swarup012/solo/pkg/logger"
	"github.com/Swarup012/solo/pkg/providers"
	"github.com/Swarup012/solo/pkg/scheduler"
	"github.com/Swarup012/solo/pkg/tools"
)

func agentCmd(message, sessionKey, model string, debug bool) error {
	if sessionKey == "" {
		sessionKey = "cli:default"
	}

	if debug {
		logger.SetLevel(logger.DEBUG)
		fmt.Println("🔍 Debug mode enabled")
	}

	cfg, err := internal.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	if model != "" {
		cfg.Agents.Defaults.ModelName = model
	}

	provider, modelID, err := providers.CreateProvider(cfg)
	if err != nil {
		return fmt.Errorf("error creating provider: %w", err)
	}

	// Use the resolved model ID from provider creation
	if modelID != "" {
		cfg.Agents.Defaults.ModelName = modelID
	}

	msgBus := bus.NewMessageBus()
	defer msgBus.Close()
	agentLoop := agent.NewAgentLoop(cfg, msgBus, provider)

	// Shared scheduler drives background jobs (email dispatch, reminders)
	sched := scheduler.New()
	ctx, cancelSched := context.WithCancel(context.Background())
	defer cancelSched()
	sched.Run(ctx)

	// ── Register Tools ─────────────────────────────────────────────────────
	// Google tools (Calendar + Gmail)
	if cfg.Integrations.Google != nil && cfg.Integrations.Google.ClientID != "" {
		googleClients, gErr := integrations.InitGoogle(cfg)
		if gErr == nil && googleClients != nil {
			calendarTool := tools.NewGoogleCalendarTool(googleClients.HTTPClient)
			agentLoop.RegisterTool(calendarTool)

			gmailTool := tools.NewGmailTool(googleClients.HTTPClient)
			// Wire email queue + dispatcher so scheduled emails are actually sent
			if eq, eqErr := scheduler.NewEmailQueue(scheduler.DefaultDBPath()); eqErr == nil {
				gmailTool.SetScheduler(eq)

				// EmailSender func wraps the gmail tool's SendImmediate
				emailSender := func(sendCtx context.Context, to []string, subject, body string, isHTML bool, cc []string) error {
					return gmailTool.SendImmediate(sendCtx, to, subject, body, isHTML, cc)
				}

				disp := scheduler.NewDispatcher(eq, emailSender, nil, msgBus, nil)
				sched.Register("email_dispatch", 30*time.Second, disp.DispatchPending)
			}
			agentLoop.RegisterTool(gmailTool)
		}
	}

	// Todoist tool
	if cfg.Integrations.Todoist != nil && cfg.Integrations.Todoist.APIToken != "" {
		agentLoop.RegisterTool(tools.NewTodoistTool(cfg.Integrations.Todoist.APIToken))
	}

	// RSS tool
	rssDBPath := scheduler.DefaultDBPath()
	agentLoop.RegisterTool(tools.NewRSSFeedTool(rssDBPath))

	// Cron tool
	workspace := cfg.WorkspacePath()
	if workspace == "" {
		home, _ := os.UserHomeDir()
		workspace = filepath.Join(home, ".octa", "workspace")
	}
	cronStorePath := filepath.Join(workspace, "cron", "jobs.json")
	cronService := cron.NewCronService(cronStorePath, nil)
	cronTool, cronErr := tools.NewCronTool(cronService, agentLoop, msgBus, workspace, cfg.Agents.Defaults.RestrictToWorkspace, 0, cfg)
	if cronErr == nil {
		// Set CLI session context so cron jobs know where to deliver messages
		cronTool.SetContext("cli", "direct")
		agentLoop.RegisterTool(cronTool)
		cronService.SetOnJob(func(job *cron.CronJob) (string, error) {
			result := cronTool.ExecuteJob(context.Background(), job)
			return result, nil
		})
		go cronService.Start() //nolint:errcheck
	} else {
		logger.DebugCF("agent", "CronTool init failed", map[string]any{"error": cronErr.Error()})
	}

	// Print agent startup info (only for interactive mode)
	startupInfo := agentLoop.GetStartupInfo()
	logger.InfoCF("agent", "Agent initialized",
		map[string]any{
			"tools_count":      startupInfo["tools"].(map[string]any)["count"],
			"skills_total":     startupInfo["skills"].(map[string]any)["total"],
			"skills_available": startupInfo["skills"].(map[string]any)["available"],
		})

	if message != "" {
		ctx := context.Background()
		response, err := agentLoop.ProcessDirect(ctx, message, sessionKey)
		if err != nil {
			return fmt.Errorf("error processing message: %w", err)
		}
		fmt.Printf("\n%s %s\n", internal.Logo, response)
		return nil
	}

	fmt.Printf("%s Interactive mode (Ctrl+C to exit)\n\n", internal.Logo)
	interactiveMode(agentLoop, sessionKey)

	return nil
}

func interactiveMode(agentLoop *agent.AgentLoop, sessionKey string) {
	prompt := fmt.Sprintf("%s You: ", internal.Logo)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          prompt,
		HistoryFile:     filepath.Join(os.TempDir(), ".octa_history"),
		HistoryLimit:    100,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		fmt.Printf("Error initializing readline: %v\n", err)
		fmt.Println("Falling back to simple input mode...")
		simpleInteractiveMode(agentLoop, sessionKey)
		return
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt || err == io.EOF {
				fmt.Println("\nGoodbye!")
				return
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return
		}

		ctx := context.Background()
		response, err := agentLoop.ProcessDirect(ctx, input, sessionKey)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("\n%s %s\n\n", internal.Logo, response)
	}
}

func simpleInteractiveMode(agentLoop *agent.AgentLoop, sessionKey string) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(fmt.Sprintf("%s You: ", internal.Logo))
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println("\nGoodbye!")
				return
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return
		}

		ctx := context.Background()
		response, err := agentLoop.ProcessDirect(ctx, input, sessionKey)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("\n%s %s\n\n", internal.Logo, response)
	}
}
