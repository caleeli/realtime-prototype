---
description: 
alwaysApply: true
---

## Purpose
This project builds an app that generates UI screens from natural language prompts, using output in Pug + CSS notation, rendered in Vue with BootstrapVue components and optional extra components.

## Product objective
- Receive a UI `prompt`.
- Generate `pug` and `css` with an economical model (`Llama 3.1 8B`).
- Validate and compile the output to show immediate preview.
- Allow use of the full BootstrapVueNext catalog and additional components.

## Mandatory stack
- Frontend: Vue + Vite + BootstrapVue.
- Backend: Golang (generation, validation, compilation, and caching API).
- AI engine: `Llama 3.1 8B` model in cerebras API.
- Future optional: Go for gateway/extreme performance, not for MVP.

## Architecture rules
- The LLM must return only `pug` and `css`.
- The backend derives `used_components` and `deps` by parsing Pug.
- Do not trust component metadata sent by the model.
- All critical validation occurs in the backend.
- The frontend never executes HTML without prior sanitization.

## Component scope
- Mandatory base: full coverage of a pinned BootstrapVueNext version.
- Mandatory extras for demo:
- `advanced-inputs`: DateRangePicker, AsyncMultiSelect, InputMask.
- `files`: DropzoneUploader.
- Extras are enabled by versioned packs.

## Generation contract (LLM -> backend)
- Minimum input:
- `prompt: string`
- `context: { locale, theme, enabledPacks, targetDensity }`
- Minimum output:
- `pug: string`
- `css: string`
- If output does not match format, it is invalid and is retried with a repair prompt.

## Execution pipeline
1. Receive prompt.
2. Build master prompt with allowed catalog.
3. Invoke local LLM.
4. Validate Pug syntax and security rules.
5. Parse AST and extract used components.
6. Resolve dependencies from Component Registry.
7. Compile Pug to HTML.
8. Sanitize HTML/CSS.
9. Deliver payload for preview.
10. Store telemetry and cache.

## Mandatory security
- Deny dangerous tags (`script`, `iframe` not allowed, etc.).
- Deny `on*` attributes.
- Restrict inline `style` if it breaks security policy.
- Validate URLs in `href/src` against a protocol allowlist.
- Sanitize final HTML before rendering.
- Apply rate limiting by IP and API key.
- Log prompt and response audit trail (without secrets).

## Performance and efficiency
- Use model quantization appropriate for hardware.
- Model warmup on service startup.
- Cache by hash of `prompt + packs + catalog version + model version`.
- Controlled retries with strict timeout.
- Optional status streaming for faster UX.
- Initial goals:
- p50 < 2.5s on medium prompts.
- p95 < 6s.
- Error rate < 2%.

## Quality and testing
- Unit tests for parser/validator.
- Contract test for LLM output.
- E2E suite for full generation-preview flow.
- Component coverage suite:
- At least one valid case per base component.
- At least one valid case per extra component.
- Regression tests when changing BootstrapVueNext or model version.

## Implementation conventions
- Keep API and schemas versioned.
- Avoid business logic in visual components.
- Centralize rules in `Component Registry`.
- Every change must include tests or explicit justification for not adding tests.
- Do not introduce dependencies without impact assessment on bundle, latency, and maintenance.

## Suggested repository structure
- `apps/web`: Vue 3 frontend.
- `apps/api`: Node.js backend.
- `packages/component-registry`: catalog and validators.
- `packages/prompt-engine`: prompt templates and repair prompts.
- `packages/render-pipeline`: parsing, compilation, and sanitization.
- `packages/shared-schemas`: types and API contracts.
- `tests`: e2e, fixtures, and regression.

## Prompting guidelines
- The system prompt should enforce strict `pug + css` output.
- Prohibit explanatory text outside expected format.
- Include short examples per layout type.
- Include list of allowed components and active packs.
- Apply a “repair” strategy when validation fails.
- Avoid unnecessarily long prompts to reduce cost/latency.

## MVP acceptance criteria
- Generates a functional screen from prompt in under 6s p95.
- Uses valid BootstrapVueNext components without rendering errors.
- Supports demo with tabs, modal, offcanvas, table, tooltip + extra DateRangePicker.
- Shows stable preview and generated code.
- Blocks unsafe content.
- Includes basic latency and error metrics.

## MVP non-goals
- Real-time multi-user collaborative editing.
- Training/fine-tuning a custom model.
- Exporting to multiple frameworks.
- Vue 2 compatibility.

## Operations and observability
- Expose metrics: latency, tokens/s, cache hit ratio, validation errors.
- Structured logging by request-id.
- Health checks for API and model server.
- Basic alerts for inference outage and p95 degradation.

## Change management
- Every library update requires:
- freeze previous version,
- run full suite,
- review component changelog.
- Every master prompt change must be versioned and A/B tested.

## Definition of “done”
- Code compiles without errors.
- Relevant tests passing.
- Minimum documentation updated.
- Performance metrics not degraded.
- Risks and technical decisions recorded.

## Current implemented features
- End-to-end screen generation pipeline from prompt to `pug/css/data` with backend validation/repair and frontend rendering.
- Project-based session management (multiple projects, active screen, screen history/revisions, soft delete constraints).
- Component registry API with persistent JSON storage, enable/disable flags, CRUD, and catalog versioning.
- Prompt-context aware generation with enabled component catalog and conversation history.
- Incremental generation modes: full screen, data-only regeneration, pug-only regeneration.
- UX evaluator endpoint integration (assistant recommendations loop).
- Inspiration flow: image generation + vision analysis + conversion to UI prompt.
- Runtime rendering engine that parses Pug AST and resolves BootstrapVueNext/custom components dynamically.
- Custom/fallback component loading for demo packs (`advanced-inputs`, `files`) and chart rendering (`VueChart`).
- Flow-diagram editing model (tasks/edges, popup tasks, submit-primary edges) persisted per project.
- SQLite-backed migrations for session schema bootstrap and upgrades.
- JSON repair logic for malformed LLM responses (escaped keys/quotes/URLs in `pug/css/data` fields).
- Storybook scenarios for validating generated-screen renderer behavior.

## Source code index
- `apps/api/cmd/server/main.go`: HTTP server entrypoint; routes for generation, inspiration, UX evaluator, projects, session, and component registry.
- `apps/api/cmd/server/projectSessionStore.go`: SQLite session/project store, revisioning, snapshot assembly, project/screen CRUD rules, flow diagram persistence.
- `apps/api/cmd/server/fix_json.go`: LLM JSON repair/sanitization utilities for `pug/css/data` payloads.
- `apps/api/cmd/server/fix_json_test.go`: tests for JSON repair edge cases (escaped fields, URL preservation, fixture parsing).
- `apps/api/cmd/server/generation-system-prompt.txt`: base system prompt template for UI generation.
- `apps/api/cmd/server/ux-evaluator.txt`: system prompt template for UX recommendations.
- `apps/api/cmd/server/inspiration-conversion-prompt.txt`: prompt template to convert inspiration analysis into UI generation input.
- `apps/api/cmd/server/image-generation-prompt.txt`: prompt template used for inspiration image generation.
- `apps/api/cmd/server/test_vlm.txt`: local test prompt/content for vision-language experiments.
- `apps/api/cmd/server/test_pug_with_urls.json`: malformed/complex JSON fixture used by repair tests.
- `apps/api/internal/registry/model.go`: backend component registry domain types.
- `apps/api/internal/registry/seed.go`: backend seeded component catalog and version.
- `apps/api/internal/registry/service.go`: in-memory + persisted component registry service (list/register/update/delete/enable).
- `apps/api/internal/db/sessionmigrations/sessionmigrations.go`: migration runner and migration history tracking.
- `apps/api/internal/db/sessionmigrations/sql/0001_init_session_schema.up.sql`: initial session/project/screen schema.
- `apps/api/internal/db/sessionmigrations/sql/0002_seed_default_project.up.sql`: default project bootstrap migration.
- `apps/api/internal/db/sessionmigrations/sql/0003_add_flow_diagrams.up.sql`: flow-diagram storage migration.
- `apps/web/src/main.ts`: Vue app bootstrap, BootstrapVueNext setup, tooltip directive registration.
- `apps/web/src/App.vue`: main UI shell; builder workspace, generation actions, session/project UX, flow canvas, editors, recommendations.
- `apps/web/src/services/generationPipelineService.ts`: frontend API client + parser helpers for generation/data/pug/UX/inspiration calls.
- `apps/web/src/services/generationRenderService.ts`: generated-screen runtime compiler/renderer from parsed Pug + CSS + data.
- `apps/web/src/services/projectSessionService.ts`: frontend client for projects/sessions/screens/history/flow-diagram endpoints.
- `apps/web/src/services/componentRegistryApi.ts`: frontend client for component registry CRUD and enable/disable.
- `apps/web/src/services/componentRegistrar.ts`: utility to register enabled catalog components into Vue app and shape prompt catalog payload.
- `apps/web/src/components/charts/VueChart.ts`: chart wrapper component mapping generic chart props to `vue-chartjs` renderers.
- `apps/web/src/GeneratedScreen.stories.ts`: Storybook stories exercising Pug/CSS/data rendering and component resolution.
- `packages/component-registry/src/types.ts`: shared TypeScript catalog schemas.
- `packages/component-registry/src/seed.ts`: shared seeded catalog data for frontend/package usage.
- `packages/component-registry/src/index.ts`: package exports and utility helpers (`buildInventoryResponse`, `upsertComponent`).
- `README.md`: root runbook (env setup, backend/frontend/storybook startup).
- `docs/PROJECT_IDEA.md`: product/architecture idea document (reference spec).

## Where to find things
- `apps/web/src/App.vue`: Main orchestration surface. Very high state/event density. Must document:
  - primary navigation/workspaces and responsibilities,
  - generation lifecycle and status flags,
  - session/project/screen synchronization flow,
  - flow-diagram interactions and popup navigation behavior,
  - editing loops for `data`, `pug`, and `css` (including undo/redo stacks).
- `apps/api/cmd/server/main.go`: Core API and pipeline orchestrator. Must document:
  - endpoint map (method/path/purpose),
  - request/response contracts per generation mode,
  - validation/repair gates and failure behavior,
  - inspiration image/vision pipeline and provider switching,
  - security controls and sanitization points.
- `apps/web/src/services/generationPipelineService.ts`: Frontend generation contract layer. Must document:
  - public methods and exact input/output shapes,
  - parser/AST and import-metadata extraction path,
  - timeout/retry/error behavior (`GenerationServiceError`),
  - differences between full generation vs data-only vs pug-only flows.
- `apps/web/src/services/generationRenderService.ts`: Runtime screen renderer. Must document:
  - component resolution strategy (BootstrapVueNext, aliases, dynamic loaders),
  - tag normalization and chart-tag mapping rules,
  - unresolved component fallback policy,
  - style injection lifecycle and cleanup behavior.
- `apps/api/cmd/server/projectSessionStore.go`: Persistence and invariants core. Must document:
  - SQLite data model and migration dependencies,
  - revision/history model and active-screen semantics,
  - project/screen lifecycle constraints (default/last project rules),
  - flow-diagram storage schema and update behavior.
- `apps/web/src/services/projectSessionService.ts` (Priority P2): Session API client façade. Must document:
  - endpoint-to-method mapping,
  - payload contracts for state saves/history loads/flow diagram updates,
  - project scoping through `projectId` query propagation.

<!-- CODEGRAPH_START -->
## CodeGraph

This project has a CodeGraph MCP server (`codegraph_*` tools) configured. CodeGraph is a tree-sitter-parsed knowledge graph of every symbol, edge, and file. Reads are sub-millisecond and return structural information grep cannot.

### When to prefer codegraph over native search

Use codegraph for **structural** questions — what calls what, what would break, where is X defined, what is X's signature. Use native grep/read only for **literal text** queries (string contents, comments, log messages) or after you already have a specific file open.

| Question | Tool |
|---|---|
| "Where is X defined?" / "Find symbol named X" | `codegraph_search` |
| "What calls function Y?" | `codegraph_callers` |
| "What does Y call?" | `codegraph_callees` |
| "What would break if I changed Z?" | `codegraph_impact` |
| "Show me Y's signature / source / docstring" | `codegraph_node` |
| "Give me focused context for a task/area" | `codegraph_context` |
| "Survey an unfamiliar module/topic" | `codegraph_explore` |
| "What files exist under path/" | `codegraph_files` |
| "Is the index healthy?" | `codegraph_status` |

### Rules of thumb

- **Trust codegraph results.** They come from a full AST parse. Do NOT re-verify them with grep — that's slower, less accurate, and wastes context.
- **Don't grep first** when looking up a symbol by name. `codegraph_search` is faster and returns kind + location + signature in one call.
- **Don't chain `codegraph_search` + `codegraph_node`** when you just want context — `codegraph_context` is one call.
- **`codegraph_explore` is the heavy hitter** for unfamiliar areas — it returns full source from all relevant files in one call, but is token-heavy. If your harness supports parallel subagents (e.g., Claude Code's Task tool), spawn one for explore-class questions to keep main session context clean.
- **Index lag**: the file watcher debounces ~500ms behind writes; don't re-query immediately after editing a file in the same turn.

### If `.codegraph/` doesn't exist

The MCP server returns "not initialized." Ask the user: *"I notice this project doesn't have CodeGraph initialized. Want me to run `codegraph init -i` to build the index?"*
<!-- CODEGRAPH_END -->
