# 🐙 Octa — Roadmap

This document outlines the planned features and improvements for Octa.

---

## 1. Core Optimization: Extreme Lightweight

- [ ] Reduce memory footprint to run on 64MB RAM devices
- [ ] Optimize SQLite usage and connection pooling
- [ ] Lazy-load all integrations (Calendar, Gmail, etc.)
- [ ] Binary size reduction via build tags

---

## 2. Security Hardening

- [ ] Input sanitization and prompt injection defense
- [ ] Sandboxed shell execution (seccomp/AppArmor)
- [ ] Secrets manager integration (Vault, 1Password)
- [ ] Per-user authentication for gateway channels
- [ ] Rate limiting per channel/user

---

## 3. Connectivity: More Channels & Providers

- [ ] WhatsApp Native support
- [ ] Matrix/Element channel
- [ ] More LLM providers (Mistral, Groq, Cohere)
- [ ] Skill marketplace (install skills from GitHub)
- [ ] MCP (Model Context Protocol) support

---

## 4. Advanced Agent Capabilities

- [ ] Browser automation (Playwright integration)
- [ ] Multi-agent collaboration (spawn and coordinate subagents)
- [ ] Long-term memory with vector search
- [ ] Proactive notifications (price alerts, news summaries, etc.)
- [ ] Voice input/output support

---

## 5. Developer Experience

- [ ] `octa dev` hot-reload mode for skill development
- [ ] Comprehensive API documentation
- [ ] Docker Compose with all services pre-configured
- [ ] One-line install script (`curl | bash`)
- [ ] GitHub Actions CI/CD pipeline

---

## 6. Productivity Tools

- [ ] Notion integration
- [ ] Obsidian / local markdown notes
- [ ] GitHub issues and PRs management
- [ ] Spotify / music control
- [ ] Home automation (Home Assistant)

---

## 7. Community

- [ ] Octa skill hub (community-contributed skills)
- [ ] Discord community server
- [ ] Video tutorials and demos
- [ ] Translations (README in multiple languages)

---

> Have an idea? [Open a feature request](../../issues/new?template=feature_request.md) 🐙
