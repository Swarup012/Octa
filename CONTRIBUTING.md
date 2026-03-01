# Contributing to Octa 🐙

Thank you for your interest in contributing to **Octa**! We welcome all contributions — bug reports, feature requests, documentation improvements, and code changes.

---

## Code of Conduct

Be respectful, inclusive, and welcoming. We follow the [Contributor Covenant](https://www.contributor-covenant.org/).

---

## Ways to Contribute

- 🐛 **Bug Reports** — Open an issue with steps to reproduce
- 💡 **Feature Requests** — Open an issue with your idea
- 📝 **Documentation** — Improve the README, add examples
- 🔧 **Code** — Fix bugs, implement features, improve performance
- 🧪 **Testing** — Add tests, improve coverage

---

## Development Setup

**Requirements:**
- Go 1.21+
- Git

```bash
# Clone
git clone git@github.com:Swarup012/Octa.git
cd octa

# Build
make build

# Run tests
make test

# Lint
make lint
```

---

## Making Changes

### Branch Strategy

```
main          → stable, production-ready
feature/xyz   → new features
fix/xyz       → bug fixes
docs/xyz      → documentation only
```

### Commit Style

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add Notion integration tool
fix: gmail duplicate send on schedule
docs: update Google auth instructions
refactor: rename cmd/picoclaw to cmd/octa
chore: update dependencies
```

### Pull Request Process

1. Fork the repo
2. Create a branch: `git checkout -b feature/your-feature`
3. Make your changes
4. Add tests if applicable
5. Run `make test` and `make lint`
6. Open a PR with a clear description

**Keep PRs focused** — one feature or fix per PR.

---

## AI-Assisted Contributions

AI-assisted code is welcome! Please disclose it in your PR description:

| Level | Label |
|---|---|
| Fully AI-generated | `ai: full` |
| Mostly AI, human-reviewed | `ai: assisted` |
| Mostly human, AI suggestions | `ai: minimal` |

All AI-generated code must be reviewed and understood by the contributor before submitting.

---

## Project Structure

```
cmd/octa/          → CLI entry point and subcommands
pkg/agent/         → Agent loop, context, memory
pkg/tools/         → Built-in tools (Gmail, Calendar, Todoist, etc.)
pkg/providers/     → LLM provider integrations
pkg/channels/      → Messaging channel integrations
pkg/scheduler/     → Email queue and job dispatcher
pkg/cron/          → Cron job service
pkg/config/        → Configuration loading
```

---

## Questions?

- 💬 [GitHub Discussions](../../discussions)
- 🐛 [GitHub Issues](../../issues)
- 📧 Open an issue and tag it `question`

Thank you for helping make Octa better! 🐙
