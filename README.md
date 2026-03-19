This project is hevily modified from: https://github.com/sipeed/picoclaw
<div align="center">

# 🐙 Octa

**Your personal AI agent — lightweight, fast, and runs everywhere.**

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build](https://img.shields.io/github/actions/workflow/status/Swarup012/solo/build.yml?branch=main)](https://github.com/Swarup012/solo/actions)

[Features](#-features) • [Quick Start](#-quick-start) • [Configuration](#-configuration) • [Docker](#-docker) • [Tools](#-built-in-tools) • [Channels](#-messaging-channels)

</div>

---

## What is Octa?

Octa is a **personal AI agent** written in Go. It connects to your favourite LLM (Claude, Gemini, GPT-4, Llama) and gives it real tools — Google Calendar, Gmail, Todoist, RSS feeds, web search, shell access, and more.

You talk to Octa through Telegram, Discord, Slack, WhatsApp, or directly from your terminal. It remembers context, runs scheduled tasks in the background, and executes multiple tools **in parallel** for fast responses.

Inspired by [OpenClaw](https://github.com/OpenClaw-project) and [PicoClaw](https://github.com/PicoClaw-project). Built for people who want a capable personal assistant that runs on their own machine, with their own API keys, under their own control.

---

## ✨ Features

- 🧠 **Multi-provider LLM** — Claude, Gemini, GPT-4, OpenRouter, Llama, and more
- ⚡ **Parallel tool execution** — multiple tools run simultaneously in one response
- 📅 **Google Calendar** — create, list, update, delete events + Google Meet
- 📧 **Gmail** — send, schedule, search, read emails
- ✅ **Todoist** — full task management (create, complete, bulk operations)
- 📰 **RSS Feed Reader** — subscribe, fetch, notify on new articles
- 🌐 **Web Search & Fetch** — search the web and read pages
- 🖥️ **Shell** — run terminal commands
- 💬 **Multiple channels** — Telegram, Discord, Slack, WhatsApp, DingTalk, Feishu, LINE
- ⏰ **Background scheduler** — email dispatch, meeting reminders, RSS fetch
- 🧩 **Skills system** — install community skills to extend capabilities
- 🐳 **Docker ready** — run as a container in seconds

---

## 🚀 Quick Start

### Prerequisites
- Go 1.21 or later
- Git
- An API key for at least one LLM provider (Claude, Gemini, OpenRouter, etc.)

### 1. Clone and build

```bash
git clone https://github.com/Swarup012/solo.git
cd solo
make build
make install
```

This installs `octa` to `~/.local/bin/octa`. Make sure `~/.local/bin` is in your PATH:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

### 2. Create your config

```bash
mkdir -p ~/.octa
cp config/config.example.json ~/.octa/config.json
```

Edit `~/.octa/config.json` with your API key. Minimum config to get started:

```json
{
  "agents": {
    "defaults": {
      "model": "gemini-2.0-flash",
      "max_tokens": 8192,
      "max_tool_iterations": 20
    }
  },
  "model_list": [
    {
      "model_name": "gemini-2.0-flash",
      "api_base": "https://generativelanguage.googleapis.com/v1beta/openai/",
      "api_key": "YOUR_GEMINI_API_KEY",
      "model": "gemini-2.0-flash"
    }
  ]
}
```

### 3. Run

```bash
# Interactive terminal session
octa agent

# One-shot query
octa agent -m "what's the weather like today?"

# Start the gateway (for Telegram/Discord/Slack bots)
octa gateway
```

---

## 🤖 LLM Providers

Octa supports any OpenAI-compatible API. Add entries to `model_list` in your config:

### Google Gemini (free tier available)

```json
{
  "model_name": "gemini-2.0-flash",
  "api_base": "https://generativelanguage.googleapis.com/v1beta/openai/",
  "api_key": "YOUR_GEMINI_API_KEY",
  "model": "gemini-2.0-flash"
}
```

Get your key: https://aistudio.google.com/app/apikey

### Anthropic Claude

```json
{
  "model_name": "claude-sonnet",
  "api_key": "sk-ant-YOUR_KEY",
  "model": "claude-sonnet-4-5"
}
```

### OpenRouter (access Claude, GPT-4, Gemini with one key)

```json
{
  "model_name": "openrouter",
  "api_base": "https://openrouter.ai/api/v1",
  "api_key": "sk-or-v1-YOUR_KEY",
  "model": "anthropic/claude-sonnet-4-5"
}
```

Get your key: https://openrouter.ai

### Ollama (local models — no API key needed)

```json
{
  "model_name": "llama3",
  "api_base": "http://localhost:11434/v1",
  "api_key": "ollama",
  "model": "llama3.1:8b"
}
```

---

## 🛠️ Built-in Tools

| Tool | What it does |
|------|-------------|
| `google_calendar` | Create, list, update, delete calendar events. Adds Google Meet by default. |
| `gmail` | Send immediate or scheduled emails. Search, read inbox. |
| `todoist` | Full task management — create (bulk), list, complete, delete, update. |
| `rss` | Subscribe to RSS/Atom feeds. Auto-fetches every 30 min in gateway mode. |
| `web_search` | Search the web using configured search API. |
| `web_fetch` | Fetch and read the content of any URL. |
| `shell` | Run shell commands on your machine. |
| `message` | Send a message to the user from within a tool chain. |
| `spawn` | Spawn a subagent to handle a subtask asynchronously. |
| `cron` | Schedule recurring tasks. |

---

## 📅 Google Calendar + Gmail Setup

### 1. Create Google OAuth credentials

1. Go to [Google Cloud Console](https://console.cloud.google.com)
2. Create a new project (or use existing)
3. Enable **Google Calendar API** and **Gmail API**
4. Go to **APIs & Services → Credentials → Create OAuth 2.0 Client ID**
5. Application type: **Desktop App**
6. Download the credentials

### 2. Add to config

```json
{
  "integrations": {
    "google": {
      "client_id": "YOUR_CLIENT_ID.apps.googleusercontent.com",
      "client_secret": "YOUR_CLIENT_SECRET",
      "redirect_uri": "http://127.0.0.1:8080/oauth/callback",
      "token_file": "~/.octa/tokens/google.json",
      "scopes": [
        "https://www.googleapis.com/auth/calendar",
        "https://www.googleapis.com/auth/gmail.modify"
      ]
    }
  }
}
```

### 3. Authenticate

```bash
octa auth google
```

This opens a browser for OAuth. After approving, the token is saved to `~/.octa/tokens/google.json`.

---

## ✅ Todoist Setup

1. Go to https://todoist.com/app/settings/integrations → **API token**
2. Add to config:

```json
{
  "integrations": {
    "todoist": {
      "api_token": "YOUR_TODOIST_API_TOKEN"
    }
  }
}
```

No OAuth needed — just the token.

---

## 💬 Messaging Channels

Run Octa as a bot on any of these platforms:

### Telegram

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "YOUR_BOT_TOKEN",
      "allowed_users": ["your_username"]
    }
  }
}
```

Get a bot token from [@BotFather](https://t.me/BotFather).

### Discord

```json
{
  "channels": {
    "discord": {
      "enabled": true,
      "token": "YOUR_DISCORD_BOT_TOKEN",
      "allowed_channel_ids": ["CHANNEL_ID"]
    }
  }
}
```

### Slack

```json
{
  "channels": {
    "slack": {
      "enabled": true,
      "bot_token": "xoxb-YOUR-TOKEN",
      "app_token": "xapp-YOUR-TOKEN"
    }
  }
}
```

Other supported channels: WhatsApp, DingTalk, Feishu, LINE, WeCom, QQ, OneBot.

---

## 🐳 Docker

### Option 1 — Gateway bot (long-running, recommended for Telegram/Discord)

```bash
# 1. Copy your config
mkdir -p config
cp ~/.octa/config.json config/config.json

# 2. Start the gateway
docker compose --profile gateway up -d
```

Logs:
```bash
docker compose logs -f octa-gateway
```

Stop:
```bash
docker compose --profile gateway down
```

### Option 2 — One-shot agent query

```bash
docker compose --profile agent run --rm octa-agent -m "what tasks do I have today?"
```

### Option 3 — Build and run manually

```bash
# Build the image
docker build -t octa .

# Run gateway
docker run -d \
  --name octa-gateway \
  --restart unless-stopped \
  -v ~/.octa/config.json:/home/octa/.octa/config.json:ro \
  -v octa-workspace:/home/octa/.octa/workspace \
  octa gateway

# Run one-shot query
docker run --rm \
  -v ~/.octa/config.json:/home/octa/.octa/config.json:ro \
  octa agent -m "hello!"
```

### Docker config tips

- Mount your `config.json` as read-only (`:ro`)
- Use a named volume for the workspace so sessions and memory persist across restarts
- For Google OAuth: mount your tokens directory:
  ```bash
  -v ~/.octa/tokens:/home/octa/.octa/tokens:ro
  ```

---

## 📁 Directory Structure

```
~/.octa/
├── config.json          # Your configuration
├── tokens/
│   └── google.json      # Google OAuth token
├── data/
│   └── scheduler.db     # SQLite: email queue + RSS cache
└── workspace/
    ├── AGENT.md         # Agent instructions + tool rules
    ├── IDENTITY.md      # Agent identity
    ├── SOUL.md          # Agent personality
    ├── USER.md          # Your profile and preferences
    ├── TOOLS.md         # Tool calling rules
    ├── memory/          # Long-term memory
    └── sessions/        # Conversation history
```

---

## ⚙️ Configuration Reference

Full example: [`config/config.example.json`](config/config.example.json)

### Minimal config

```json
{
  "agents": {
    "defaults": {
      "model": "gemini-2.0-flash",
      "max_tokens": 8192,
      "max_tool_iterations": 20
    }
  },
  "model_list": [
    {
      "model_name": "gemini-2.0-flash",
      "api_base": "https://generativelanguage.googleapis.com/v1beta/openai/",
      "api_key": "YOUR_API_KEY",
      "model": "gemini-2.0-flash"
    }
  ]
}
```

### With Telegram bot + Google tools + Todoist

```json
{
  "agents": {
    "defaults": {
      "model": "gemini-2.0-flash",
      "max_tokens": 8192,
      "max_tool_iterations": 20
    }
  },
  "model_list": [
    {
      "model_name": "gemini-2.0-flash",
      "api_base": "https://generativelanguage.googleapis.com/v1beta/openai/",
      "api_key": "YOUR_GEMINI_KEY",
      "model": "gemini-2.0-flash"
    }
  ],
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "YOUR_TELEGRAM_BOT_TOKEN",
      "allowed_users": ["your_telegram_username"]
    }
  },
  "integrations": {
    "google": {
      "client_id": "YOUR_GOOGLE_CLIENT_ID.apps.googleusercontent.com",
      "client_secret": "YOUR_GOOGLE_CLIENT_SECRET",
      "redirect_uri": "http://127.0.0.1:8080/oauth/callback",
      "token_file": "~/.octa/tokens/google.json",
      "scopes": [
        "https://www.googleapis.com/auth/calendar",
        "https://www.googleapis.com/auth/gmail.modify"
      ]
    },
    "todoist": {
      "api_token": "YOUR_TODOIST_API_TOKEN"
    }
  }
}
```

---

## 🖥️ CLI Commands

```bash
octa agent              # Start interactive terminal session
octa agent -m "msg"     # One-shot message
octa agent --model X    # Use a specific model for this session
octa gateway            # Start gateway (Telegram/Discord/Slack bot + background jobs)
octa auth google        # Run Google OAuth flow
octa status             # Show status
octa cron               # Manage scheduled tasks
octa skills             # Manage skills (install, list, remove)
octa migrate            # Migrate from OpenClaw config format
octa version            # Show version
```

---

## 🧩 Skills

Skills are markdown files that extend Octa's capabilities with custom instructions and scripts.

```bash
# Install a skill from the community hub
octa skills install weather

# List installed skills
octa skills list

# Remove a skill
octa skills remove weather
```

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────┐
│                   Channels                   │
│  Telegram │ Discord │ Slack │ CLI │ WhatsApp │
└──────────────────┬──────────────────────────┘
                   │ messages
                   ▼
┌──────────────────────────────────────────────┐
│              Message Bus (pkg/bus)            │
└──────────────────┬───────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────┐
│           Agent Loop (pkg/agent)             │
│  Session → System Prompt → LLM → Tool Calls  │
└──────────────────┬───────────────────────────┘
                   │ parallel execution
        ┌──────────┼──────────┐
        ▼          ▼          ▼
   Calendar      Gmail     Todoist    RSS ...
                   
┌──────────────────────────────────────────────┐
│         Shared Scheduler (pkg/scheduler)      │
│  email_dispatch │ meeting_reminder │ rss_fetch│
└──────────────────────────────────────────────┘

┌──────────────────────────────────────────────┐
│         Shared SQLite Pool (pkg/db)           │
│      email queue + RSS cache (sync.Once)      │
└──────────────────────────────────────────────┘
```

---

## 🤝 Contributing

Contributions are welcome! Please read the contributing guidelines before submitting a PR.

```bash
# Run tests
make test

# Run linter
make lint

# Format code
make fmt
```

---

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.

---

<div align="center">
Made with 🐙 by <a href="https://github.com/Swarup012">Swarup</a>
</div>
