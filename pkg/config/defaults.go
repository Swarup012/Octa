// Octa - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Octa contributors

package config

// DefaultConfig returns the default configuration for Octa.
// This matches config/config.example.json so onboard generates the same config.
func DefaultConfig() *Config {
	return &Config{
		Session: SessionConfig{
			DMScope: "main",
		},
		Agents: AgentsConfig{
			Defaults: AgentDefaults{
				Workspace:           "~/.octa/workspace",
				RestrictToWorkspace: true,
				Provider:            "",
				Model:               "claude-sonnet-4.6",
				MaxTokens:           8192,
				Temperature:         nil,
				MaxToolIterations:   20,
			},
		},
		Bindings: []AgentBinding{},
		Channels: ChannelsConfig{
			WhatsApp: WhatsAppConfig{
				Enabled:          false,
				BridgeURL:        "ws://localhost:3001",
				UseNative:        false,
				SessionStorePath: "",
				AllowFrom:        FlexibleStringSlice{},
			},
			Telegram: TelegramConfig{
				Enabled:   true,
				Token:     "",
				AllowFrom: FlexibleStringSlice{"your_telegram_id"},
				Typing:    TypingConfig{Enabled: true},
				Placeholder: PlaceholderConfig{
					Enabled: true,
					Text:    "Thinking... 💭",
				},
			},
			Feishu: FeishuConfig{
				Enabled:           false,
				AppID:             "",
				AppSecret:         "",
				EncryptKey:        "",
				VerificationToken: "",
				AllowFrom:         FlexibleStringSlice{},
			},
			Discord: DiscordConfig{
				Enabled:     true,
				Token:       "",
				AllowFrom:   FlexibleStringSlice{""},
				MentionOnly: true,
			},
			MaixCam: MaixCamConfig{
				Enabled:   false,
				Host:      "0.0.0.0",
				Port:      18790,
				AllowFrom: FlexibleStringSlice{},
			},
			QQ: QQConfig{
				Enabled:   false,
				AppID:     "",
				AppSecret: "",
				AllowFrom: FlexibleStringSlice{},
			},
			DingTalk: DingTalkConfig{
				Enabled:      false,
				ClientID:     "",
				ClientSecret: "",
				AllowFrom:    FlexibleStringSlice{},
			},
			Slack: SlackConfig{
				Enabled:   false,
				BotToken:  "",
				AppToken:  "",
				AllowFrom: FlexibleStringSlice{},
			},
			LINE: LINEConfig{
				Enabled:            false,
				ChannelSecret:      "",
				ChannelAccessToken: "",
				WebhookHost:        "0.0.0.0",
				WebhookPort:        18791,
				WebhookPath:        "/webhook/line",
				AllowFrom:          FlexibleStringSlice{},
				GroupTrigger:       GroupTriggerConfig{MentionOnly: true},
			},
			OneBot: OneBotConfig{
				Enabled:            false,
				WSUrl:              "ws://127.0.0.1:3001",
				AccessToken:        "",
				ReconnectInterval:  5,
				GroupTriggerPrefix: []string{},
				AllowFrom:          FlexibleStringSlice{},
			},
			WeCom: WeComConfig{
				Enabled:        false,
				Token:          "",
				EncodingAESKey: "",
				WebhookURL:     "",
				WebhookHost:    "0.0.0.0",
				WebhookPort:    18793,
				WebhookPath:    "/webhook/wecom",
				AllowFrom:      FlexibleStringSlice{},
				ReplyTimeout:   5,
			},
			WeComApp: WeComAppConfig{
				Enabled:        false,
				CorpID:         "",
				CorpSecret:     "",
				AgentID:        0,
				Token:          "",
				EncodingAESKey: "",
				WebhookHost:    "0.0.0.0",
				WebhookPort:    18792,
				WebhookPath:    "/webhook/wecom-app",
				AllowFrom:      FlexibleStringSlice{},
				ReplyTimeout:   5,
			},
			Pico: PicoConfig{
				Enabled:        false,
				Token:          "",
				PingInterval:   30,
				ReadTimeout:    60,
				WriteTimeout:   10,
				MaxConnections: 100,
				AllowFrom:      FlexibleStringSlice{},
			},
		},
		Providers: ProvidersConfig{
			OpenAI: OpenAIProviderConfig{WebSearch: true},
		},
		ModelList: []ModelConfig{
			{
				ModelName: "glm-4.7",
				Model:     "zhipu/glm-4.7",
				APIBase:   "https://open.bigmodel.cn/api/paas/v4",
				APIKey:    "",
			},
			{
				ModelName: "gpt-5.2",
				Model:     "openai/gpt-5.2",
				APIBase:   "https://api.openai.com/v1",
				APIKey:    "",
			},
			{
				ModelName: "claude-sonnet-4.6",
				Model:     "anthropic/claude-sonnet-4.6",
				APIBase:   "https://api.anthropic.com/v1",
				APIKey:    "",
			},
			{
				ModelName: "deepseek-chat",
				Model:     "deepseek/deepseek-chat",
				APIBase:   "https://api.deepseek.com/v1",
				APIKey:    "",
			},
			{
				ModelName: "gemini-test",
				Model:     "gemini/gemini-2.5-flash",
				APIBase:   "https://generativelanguage.googleapis.com/v1beta",
				APIKey:    "",
			},
			{
				ModelName: "qwen-plus",
				Model:     "qwen/qwen-plus",
				APIBase:   "https://dashscope.aliyuncs.com/compatible-mode/v1",
				APIKey:    "",
			},
			{
				ModelName: "moonshot-v1-8k",
				Model:     "moonshot/moonshot-v1-8k",
				APIBase:   "https://api.moonshot.cn/v1",
				APIKey:    "",
			},
			{
				ModelName: "llama-3",
				Model:     "openai/meta/llama-3.1-70b-instruct",
				APIBase:   "https://integrate.api.nvidia.com/v1",
				APIKey:    "",
			},
			{
				ModelName: "openrouter-free",
				Model:     "openai/openrouter/free",
				APIBase:   "https://openrouter.ai/api/v1",
				APIKey:    "",
			},
			{
				ModelName: "openrouter-gpt-5.2",
				Model:     "openrouter/openai/gpt-5.2",
				APIBase:   "https://openrouter.ai/api/v1",
				APIKey:    "",
			},
			{
				ModelName: "nemotron-4-340b",
				Model:     "nvidia/nemotron-4-340b-instruct",
				APIBase:   "https://integrate.api.nvidia.com/v1",
				APIKey:    "",
			},
			{
				ModelName: "cerebras-llama-3.3-70b",
				Model:     "cerebras/llama-3.3-70b",
				APIBase:   "https://api.cerebras.ai/v1",
				APIKey:    "",
			},
			{
				ModelName: "doubao-pro",
				Model:     "volcengine/doubao-pro-32k",
				APIBase:   "https://ark.cn-beijing.volces.com/api/v3",
				APIKey:    "",
			},
			{
				ModelName: "deepseek-v3",
				Model:     "shengsuanyun/deepseek-v3",
				APIBase:   "https://api.shengsuanyun.com/v1",
				APIKey:    "",
			},
			{
				ModelName:  "gemini",
				Model:      "gemini/gemini-2.5-pro",
				AuthMethod: "oauth",
			},
			{
				ModelName:  "copilot-gpt-5.2",
				Model:      "github-copilot/gpt-5.2",
				APIBase:    "http://localhost:4321",
				AuthMethod: "oauth",
			},
			{
				ModelName: "llama3",
				Model:     "ollama/llama3",
				APIBase:   "http://localhost:11434/v1",
				APIKey:    "ollama",
			},
			{
				ModelName: "mistral-small",
				Model:     "mistral/mistral-small-latest",
				APIBase:   "https://api.mistral.ai/v1",
				APIKey:    "",
			},
			{
				ModelName: "local-model",
				Model:     "vllm/custom-model",
				APIBase:   "http://localhost:8000/v1",
				APIKey:    "",
			},
		},
		Gateway: GatewayConfig{
			Host: "127.0.0.1",
			Port: 18790,
		},
		Tools: ToolsConfig{
			MediaCleanup: MediaCleanupConfig{
				Enabled:  true,
				MaxAge:   30,
				Interval: 5,
			},
			Web: WebToolsConfig{
				Proxy: "",
				Brave: BraveConfig{
					Enabled:    false,
					APIKey:     "",
					MaxResults: 5,
				},
				DuckDuckGo: DuckDuckGoConfig{
					Enabled:    true,
					MaxResults: 5,
				},
				Perplexity: PerplexityConfig{
					Enabled:    false,
					APIKey:     "",
					MaxResults: 5,
				},
			},
			Cron: CronToolsConfig{
				ExecTimeoutMinutes: 5,
			},
			Exec: ExecConfig{
				EnableDenyPatterns: true,
			},
			Skills: SkillsToolsConfig{
				Registries: SkillsRegistriesConfig{
					ClawHub: ClawHubRegistryConfig{
						Enabled: true,
						BaseURL: "https://clawhub.ai",
					},
				},
				MaxConcurrentSearches: 2,
				SearchCache: SearchCacheConfig{
					MaxSize:    50,
					TTLSeconds: 300,
				},
			},
		},
		Heartbeat: HeartbeatConfig{
			Enabled:  true,
			Interval: 30,
		},
		Devices: DevicesConfig{
			Enabled:    false,
			MonitorUSB: true,
		},
		Integrations: IntegrationsConfig{
			Google: &GoogleConfig{
				ClientID:     "your_google_client_id",
				ClientSecret: "your_google_client_secret",
				RedirectURI:  "http://127.0.0.1:8080/oauth/callback",
				TokenFile:    "",
				Scopes: []string{
					"https://www.googleapis.com/auth/calendar",
					"https://www.googleapis.com/auth/gmail.modify",
				},
			},
			Todoist: &TodoistConfig{
				APIToken: "your_todoist_api_token",
			},
		},
	}
}
