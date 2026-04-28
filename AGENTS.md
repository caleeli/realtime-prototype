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