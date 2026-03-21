# Issue #783 Investigation & Fix Plan

## 1. Problem (Confirmed)

- **Symptom:** When `agents.*.model.primary/fallbacks` uses a `model_name` alias (e.g., `step-3.5-flash`), the fallback chain parses the alias as a raw `provider/model` string, resulting in an empty `provider` or incorrect `model`.
- **Root cause:** `ResolveCandidates` only calls `ParseModelRef` on the string — it does not first resolve the alias through `model_list`.
- **Impact:**
  - Fallback may send the alias directly to an OpenAI-compatible provider, triggering `Unknown Model`.
  - When `defaults.provider` is empty, logs show `provider=` with an empty value.

## 2. Goals

- Fix fallback candidate resolution: resolve aliases via `model_list` first.
- Backward compatible: if the alias is not found in `model_list`, fall back to the existing `ParseModelRef` behavior.
- Add tests: cover aliases, nested model paths (e.g., `openrouter/stepfun/...`), and empty default provider.
- Code style: match existing repository conventions (naming, error handling, test structure).

## 3. Research Findings (Completed)

- [x] Reviewed OpenAI-compatible gateway (e.g., OpenRouter) recommendations for the `model` field.
- [x] Reviewed multi-provider/fallback design best practices (candidate resolution, observability).
- [x] Mapped external recommendations to actionable constraints for this repository.

Key takeaways (from OpenRouter, LiteLLM, Cloudflare AI Gateway docs):

- Prefer explicit configuration over string-splitting to infer the provider.
- Preserve full model path semantics for gateway identifiers to avoid `Unknown Model`.
- Fallback and primary should reuse the same resolution strategy — avoid "primary works, fallback breaks."

References:

- [OpenRouter Provider Routing](https://openrouter.ai/docs/guides/routing/provider-selection)
- [OpenRouter Model Fallbacks](https://openrouter.ai/docs/guides/routing/model-fallbacks)
- [OpenRouter Chat Completion API](https://openrouter.ai/docs/api-reference/chat-completion)
- [LiteLLM Router Architecture](https://docs.litellm.ai/docs/router_architecture)
- [Cloudflare AI Gateway Chat Completion](https://developers.cloudflare.com/ai-gateway/usage/chat-completion/)

Actionable constraints for this repo:

- Resolve `model_name → model_list.model` during fallback candidate construction.
- Preserve old parsing behavior when the alias is not in `model_list` (backward compatibility).
- Lock down "alias + nested model path + empty default provider" scenarios with new tests.

## 4. Implementation Steps (Sequential)

- [x] Step 1: Align with existing code patterns, identify minimal change surface (`pkg/agent` + `pkg/providers`).
- [x] Step 2: Implement `model_list`-based fallback candidate resolution.
- [x] Step 3: Add/update unit tests covering the issue scenarios.
- [x] Step 4: Code style review (compare against existing file conventions).
- [x] Step 5: Run quality gates (LSP + `make check`).

## 5. Execution Log

- **Status:** Complete
- **Changes made:**
  - `pkg/providers/fallback.go`: Added `ResolveCandidatesWithLookup`; `ResolveCandidates` remains backward compatible.
  - `pkg/agent/instance.go`: Before building fallback candidates, resolve aliases via `model_list`; for models without a protocol prefix, prepend default `openai/` before resolution.
  - `pkg/providers/fallback_test.go`: New tests for alias resolution and deduplication.
  - `pkg/agent/instance_test.go`: New tests for agent-side alias resolution into nested model paths and protocol-less model parsing.
- **Style check (done):** Consistent with `pkg/providers/fallback_test.go` and `pkg/providers/model_ref_test.go`.
- **Quality verification (done):** `make generate` then `make check` — all passed.
