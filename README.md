<div align="center">

  <h1>🐙 Octa — Your Personal AI Assistant</h1>

  <h3>Google Calendar · Gmail · Todoist · RSS · Telegram · Cron Jobs · Web Search</h3>

  <p>
    <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go">
    <img src="https://img.shields.io/badge/Arch-x86__64%2C%20ARM64-blue" alt="Hardware">
    <img src="https://img.shields.io/badge/license-MIT-green" alt="License">
  </p>

</div>

---

**Octa** is a lightweight, self-hosted personal AI assistant written in Go. It connects to your productivity tools — Google Calendar, Gmail, Todoist, RSS feeds — and lets you control everything through natural language, from a CLI, Telegram, Discord, or any other channel.

Built for people who want a personal AI that actually does things — not just talks about them.

---

## ✨ Features

- 🗓️ **Google Calendar** — create, list, find, update, delete events and meetings
- 📧 **Gmail** — send, read, search, schedule emails
- ✅ **Todoist** — add tasks, bulk-add, complete, delete, update (single API call for bulk)
- 📰 **RSS** — subscribe to feeds, read and mark articles
- ⏰ **Cron Jobs** — schedule reminders and recurring tasks in natural language
- ⚡ **Parallel Tool Execution** — multiple tools run concurrently using goroutines
- 🤖 **Multi-Agent** — spawn subagents for complex tasks
- 💬 **Multi-Channel** — Telegram, Discord, Slack, WhatsApp, LINE, DingTalk, Feishu, WeChat
- 🔐 **Google OAuth** — browser-based login flow built-in (`octa auth google`)
- 🧠 **Persistent Memory** — session history and workspace memory

---

## 📦 Installation

### Option 1: Build from Source (Recommended)

**Requirements:** Go 1.21+

#### Linux / macOS

```bash
# Clone the repo
git clone https://github.com/Swarup012/Octa.git
cd Octa

# Download dependencies
make deps

# Build
make build

# Install globally
sudo make install
```

#### Windows

On Windows, use `go build` directly (or use [WSL](https://learn.microsoft.com/en-us/windows/wsl/install)):

```powershell
# Clone the repo
git clone https://github.com/Swarup012/Octa.git
cd Octa

# Download dependencies
go mod download

# Build
go build -o build\octa.exe .\cmd\octa

# Add to PATH (run as Administrator) or copy manually
copy build\octa.exe C:\Windows\System32\octa.exe
```

> 💡 **Tip:** If you have WSL installed, you can use the Linux/macOS instructions instead — `make` works perfectly in WSL.


---

## ⚙️ Configuration

Octa reads its config from `~/.octa/config.json`.

```bash
# 1. Initialize workspace
octa onboard

# 2. Replace the generated config with the full example config
cp config/config.example.json ~/.octa/config.json

# 3. Edit it with your API keys and bot tokens
nano ~/.octa/config.json
```

See `config/config.example.json` for the full list of available options including Google Calendar, Gmail, Todoist, Telegram, Discord and more.

---

## 🔐 Google Authentication

To enable Google Calendar and Gmail, you need to authenticate once:

### Step 1: Create Google OAuth Credentials

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project (or use existing)
3. Enable **Gmail API** and **Google Calendar API**
4. Go to **APIs & Services → Credentials → Create OAuth Client ID**
5. Choose **Desktop App** or **Web Application**
6. Add `http://127.0.0.1:8080/oauth/callback` as an **Authorized Redirect URI**
7. Copy your `client_id` and `client_secret` into `~/.octa/config.json`

### Step 2: Authenticate

```bash
octa auth google
```

This opens your browser → log in with Google → token is saved to `~/.octa/tokens/google.json` automatically. ✅

---

## 🚀 Usage

### Interactive CLI

```bash
octa agent
```

Then just talk naturally:

```
🐙 You: add go to gym, read a book, and drink water to my todoist
🐙 You: schedule an email to john@example.com with subject "Meeting" in 10 minutes
🐙 You: what's on my calendar tomorrow?
🐙 You: remind me to take a break in 30 minutes
🐙 You: show my inbox
```

### One-Shot Command

```bash
octa agent -m "add buy groceries to my todo list"
```

### Auth Commands

```bash
octa auth login --provider openai      # Set OpenAI API key
octa auth login --provider anthropic   # Set Anthropic API key
octa auth google                        # Authenticate Google (Calendar + Gmail)
octa auth status                        # Check all auth status
```

### Cron Jobs

```bash
octa cron list          # List all scheduled jobs
octa cron disable 1     # Disable job with ID 1
octa cron enable 1      # Enable job with ID 1
octa cron remove 1      # Remove job with ID 1
```

### Skills

```bash
octa skills list            # List installed skills
octa skills install <name>  # Install a skill
octa skills remove <name>   # Remove a skill
```

---

## 🌐 Running as a Gateway (Multi-Channel)

To connect Octa to Telegram, Discord, Slack, etc., run the gateway:

```bash
octa gateway
```

Add channel config to `~/.octa/config.json`:

```json
{
  "channels": [
    {
      "type": "telegram",
      "token": "YOUR_TELEGRAM_BOT_TOKEN"
    }
  ]
}
```

---

## 🏗️ Architecture

```
octa agent
  └── AgentLoop (pkg/agent/loop.go)
        ├── Parallel Tool Execution (goroutines + WaitGroup)
        ├── Google Calendar Tool   (lazy init)
        ├── Gmail Tool             (with email scheduler)
        ├── Todoist Tool           (bulk API calls)
        ├── RSS Feed Tool          (SQLite backed)
        ├── Cron Tool              (persistent jobs)
        ├── Shell Tool             (sandboxed)
        └── Web Search Tool
```

---

## 🛠️ Supported Providers

| Provider | Models |
|---|---|
| **OpenAI** | gpt-4o, gpt-4o-mini, gpt-4-turbo, o1, o3 |
| **Anthropic** | claude-3-5-sonnet, claude-3-opus, claude-3-haiku |
| **Google Antigravity** | gemini-2.0-flash, gemini-1.5-pro |
| **OpenAI-Compatible** | Any local or cloud endpoint (Ollama, LM Studio, etc.) |

---

## 📁 Data & Storage

All data is stored locally in `~/.octa/`:

```
~/.octa/
├── config.json          # Main configuration
├── tokens/
│   └── google.json      # Google OAuth token
├── data/
│   └── scheduler.db     # Email queue + RSS feeds (SQLite)
└── workspace/
    ├── sessions/         # Conversation history
    ├── cron/             # Cron job definitions
    └── memory/           # Agent memory
```

---

## 🤝 Contributing

Pull requests are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) first.

---

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.
