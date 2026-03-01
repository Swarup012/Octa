package gateway

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/Swarup012/solo/cmd/octa/internal"
	"github.com/Swarup012/solo/pkg/agent"
	"github.com/Swarup012/solo/pkg/bus"
	"github.com/Swarup012/solo/pkg/channels"
	_ "github.com/Swarup012/solo/pkg/channels/dingtalk"
	_ "github.com/Swarup012/solo/pkg/channels/discord"
	_ "github.com/Swarup012/solo/pkg/channels/feishu"
	_ "github.com/Swarup012/solo/pkg/channels/line"
	_ "github.com/Swarup012/solo/pkg/channels/maixcam"
	_ "github.com/Swarup012/solo/pkg/channels/onebot"
	_ "github.com/Swarup012/solo/pkg/channels/pico"
	_ "github.com/Swarup012/solo/pkg/channels/qq"
	_ "github.com/Swarup012/solo/pkg/channels/slack"
	_ "github.com/Swarup012/solo/pkg/channels/telegram"
	_ "github.com/Swarup012/solo/pkg/channels/wecom"
	_ "github.com/Swarup012/solo/pkg/channels/whatsapp"
	_ "github.com/Swarup012/solo/pkg/channels/whatsapp_native"
	"github.com/Swarup012/solo/pkg/config"
	"github.com/Swarup012/solo/pkg/cron"
	"github.com/Swarup012/solo/pkg/devices"
	"github.com/Swarup012/solo/pkg/health"
	"github.com/Swarup012/solo/pkg/heartbeat"
	"github.com/Swarup012/solo/pkg/integrations"
	"github.com/Swarup012/solo/pkg/logger"
	"github.com/Swarup012/solo/pkg/media"
	"github.com/Swarup012/solo/pkg/providers"
	"github.com/Swarup012/solo/pkg/scheduler"
	"github.com/Swarup012/solo/pkg/state"
	"github.com/Swarup012/solo/pkg/tools"
)

func gatewayCmd(debug bool) error {
	if debug {
		logger.SetLevel(logger.DEBUG)
		fmt.Println("🔍 Debug mode enabled")
	}

	cfg, err := internal.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
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
	agentLoop := agent.NewAgentLoop(cfg, msgBus, provider)

	// ── Shared Scheduler ───────────────────────────────────────────────────
	sched := scheduler.New()

	// ── Google Tools (Calendar + Gmail) ────────────────────────────────────
	var emailQueue *scheduler.EmailQueue
	if cfg.Integrations.Google != nil && cfg.Integrations.Google.ClientID != "" {
		googleClients, gErr := integrations.InitGoogle(cfg)
		if gErr == nil && googleClients != nil {
			calendarTool := tools.NewGoogleCalendarTool(googleClients.HTTPClient)
			agentLoop.RegisterTool(calendarTool)

			gmailTool := tools.NewGmailTool(googleClients.HTTPClient)
			eq, eqErr := scheduler.NewEmailQueue(scheduler.DefaultDBPath())
			if eqErr == nil {
				gmailTool.SetScheduler(eq)
				emailQueue = eq
			}
			agentLoop.RegisterTool(gmailTool)

			calTool2 := tools.NewGoogleCalendarTool(googleClients.HTTPClient)
			gmailTool2 := tools.NewGmailTool(googleClients.HTTPClient)

			var calLister scheduler.CalendarUpcomingLister
			calLister = func(ctx context.Context, window time.Duration) ([]scheduler.CalendarEvent, error) {
				events, err := calTool2.ListUpcoming(ctx, window)
				if err != nil {
					return nil, err
				}
				out := make([]scheduler.CalendarEvent, 0, len(events))
				for _, e := range events {
					startRFC := ""
					if e.Start != nil {
						startRFC = e.Start.DateTime
						if startRFC == "" {
							startRFC = e.Start.Date
						}
					}
					out = append(out, scheduler.CalendarEvent{
						ID:       e.Id,
						Title:    e.Summary,
						StartRFC: startRFC,
					})
				}
				return out, nil
			}
			emailSender := scheduler.EmailSender(func(ctx context.Context, to []string, subject, body string, isHTML bool, cc []string) error {
				return gmailTool2.SendImmediate(ctx, to, subject, body, isHTML, cc)
			})
			if emailQueue != nil {
				disp := scheduler.NewDispatcher(emailQueue, emailSender, calLister, msgBus, nil)
				sched.Register("email_dispatch", 60*time.Second, disp.DispatchPending)
				sched.Register("meeting_reminder", 5*time.Minute, disp.CheckUpcomingEvents)
				fmt.Println("✓ Google tools registered (calendar + gmail + dispatcher)")
			}
		}
	}

	// ── Todoist ─────────────────────────────────────────────────────────────
	if cfg.Integrations.Todoist != nil && cfg.Integrations.Todoist.APIToken != "" {
		agentLoop.RegisterTool(tools.NewTodoistTool(cfg.Integrations.Todoist.APIToken))
		fmt.Println("✓ Todoist tool registered")
	}

	// ── RSS Feed ─────────────────────────────────────────────────────────────
	rssDBPath := scheduler.DefaultDBPath()
	agentLoop.RegisterTool(tools.NewRSSFeedTool(rssDBPath))
	rssFetcher := tools.NewRSSFetcher(rssDBPath, msgBus)
	sched.Register("rss_fetch", 30*time.Minute, rssFetcher.Fetch)
	fmt.Println("✓ RSS feed tool registered")

	// Print agent startup info
	fmt.Println("\n📦 Agent Status:")
	startupInfo := agentLoop.GetStartupInfo()
	toolsInfo := startupInfo["tools"].(map[string]any)
	skillsInfo := startupInfo["skills"].(map[string]any)
	fmt.Printf("  • Tools: %d loaded\n", toolsInfo["count"])
	fmt.Printf("  • Skills: %d/%d available\n",
		skillsInfo["available"],
		skillsInfo["total"])

	// Log to file as well
	logger.InfoCF("agent", "Agent initialized",
		map[string]any{
			"tools_count":      toolsInfo["count"],
			"skills_total":     skillsInfo["total"],
			"skills_available": skillsInfo["available"],
		})

	// Setup cron tool and service
	execTimeout := time.Duration(cfg.Tools.Cron.ExecTimeoutMinutes) * time.Minute
	cronService := setupCronTool(
		agentLoop,
		msgBus,
		cfg.WorkspacePath(),
		cfg.Agents.Defaults.RestrictToWorkspace,
		execTimeout,
		cfg,
	)

	heartbeatService := heartbeat.NewHeartbeatService(
		cfg.WorkspacePath(),
		cfg.Heartbeat.Interval,
		cfg.Heartbeat.Enabled,
	)
	heartbeatService.SetBus(msgBus)
	heartbeatService.SetHandler(func(prompt, channel, chatID string) *tools.ToolResult {
		// Use cli:direct as fallback if no valid channel
		if channel == "" || chatID == "" {
			channel, chatID = "cli", "direct"
		}
		// Use ProcessHeartbeat - no session history, each heartbeat is independent
		var response string
		response, err = agentLoop.ProcessHeartbeat(context.Background(), prompt, channel, chatID)
		if err != nil {
			return tools.ErrorResult(fmt.Sprintf("Heartbeat error: %v", err))
		}
		if response == "HEARTBEAT_OK" {
			return tools.SilentResult("Heartbeat OK")
		}
		// For heartbeat, always return silent - the subagent result will be
		// sent to user via processSystemMessage when the async task completes
		return tools.SilentResult(response)
	})

	// Create media store for file lifecycle management with TTL cleanup
	mediaStore := media.NewFileMediaStoreWithCleanup(media.MediaCleanerConfig{
		Enabled:  cfg.Tools.MediaCleanup.Enabled,
		MaxAge:   time.Duration(cfg.Tools.MediaCleanup.MaxAge) * time.Minute,
		Interval: time.Duration(cfg.Tools.MediaCleanup.Interval) * time.Minute,
	})
	mediaStore.Start()

	channelManager, err := channels.NewManager(cfg, msgBus, mediaStore)
	if err != nil {
		mediaStore.Stop()
		return fmt.Errorf("error creating channel manager: %w", err)
	}

	// Inject channel manager and media store into agent loop
	agentLoop.SetChannelManager(channelManager)
	agentLoop.SetMediaStore(mediaStore)

	enabledChannels := channelManager.GetEnabledChannels()
	if len(enabledChannels) > 0 {
		fmt.Printf("✓ Channels enabled: %s\n", enabledChannels)
	} else {
		fmt.Println("⚠ Warning: No channels enabled")
	}

	fmt.Printf("✓ Gateway started on %s:%d\n", cfg.Gateway.Host, cfg.Gateway.Port)
	fmt.Println("Press Ctrl+C to stop")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched.Run(ctx)
	fmt.Println("✓ Shared scheduler started")

	if err := cronService.Start(); err != nil {
		fmt.Printf("Error starting cron service: %v\n", err)
	}
	fmt.Println("✓ Cron service started")

	if err := heartbeatService.Start(); err != nil {
		fmt.Printf("Error starting heartbeat service: %v\n", err)
	}
	fmt.Println("✓ Heartbeat service started")

	stateManager := state.NewManager(cfg.WorkspacePath())
	deviceService := devices.NewService(devices.Config{
		Enabled:    cfg.Devices.Enabled,
		MonitorUSB: cfg.Devices.MonitorUSB,
	}, stateManager)
	deviceService.SetBus(msgBus)
	if err := deviceService.Start(ctx); err != nil {
		fmt.Printf("Error starting device service: %v\n", err)
	} else if cfg.Devices.Enabled {
		fmt.Println("✓ Device event service started")
	}

	// Setup shared HTTP server with health endpoints and webhook handlers
	healthServer := health.NewServer(cfg.Gateway.Host, cfg.Gateway.Port)
	addr := fmt.Sprintf("%s:%d", cfg.Gateway.Host, cfg.Gateway.Port)
	channelManager.SetupHTTPServer(addr, healthServer)

	if err := channelManager.StartAll(ctx); err != nil {
		fmt.Printf("Error starting channels: %v\n", err)
		return err
	}

	fmt.Printf("✓ Health endpoints available at http://%s:%d/health and /ready\n", cfg.Gateway.Host, cfg.Gateway.Port)

	go agentLoop.Run(ctx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	fmt.Println("\nShutting down...")
	if cp, ok := provider.(providers.StatefulProvider); ok {
		cp.Close()
	}
	cancel()
	sched.Stop()
	msgBus.Close()

	// Use a fresh context with timeout for graceful shutdown,
	// since the original ctx is already canceled.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	channelManager.StopAll(shutdownCtx)
	deviceService.Stop()
	heartbeatService.Stop()
	cronService.Stop()
	mediaStore.Stop()
	agentLoop.Stop()
	fmt.Println("✓ Gateway stopped")

	return nil
}

func setupCronTool(
	agentLoop *agent.AgentLoop,
	msgBus *bus.MessageBus,
	workspace string,
	restrict bool,
	execTimeout time.Duration,
	cfg *config.Config,
) *cron.CronService {
	cronStorePath := filepath.Join(workspace, "cron", "jobs.json")

	// Create cron service
	cronService := cron.NewCronService(cronStorePath, nil)

	// Create and register CronTool
	cronTool, err := tools.NewCronTool(cronService, agentLoop, msgBus, workspace, restrict, execTimeout, cfg)
	if err != nil {
		log.Fatalf("Critical error during CronTool initialization: %v", err)
	}

	agentLoop.RegisterTool(cronTool)

	// Set the onJob handler
	cronService.SetOnJob(func(job *cron.CronJob) (string, error) {
		result := cronTool.ExecuteJob(context.Background(), job)
		return result, nil
	})

	return cronService
}
