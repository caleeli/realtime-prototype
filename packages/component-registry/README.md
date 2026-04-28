# Component Registry

This package stores the shared component metadata contract between backend and frontend.

## What is included

- Shared schema (`packages/component-registry/src/types.ts`)
- Seed inventory for requested components
  - `DateRangePicker`
  - `AsyncMultiSelect`
  - `InputMask`
  - `pm-table`
  - `DropzoneUploader`
- Backend service and endpoint wiring
  - `apps/api/internal/registry/*`
  - `apps/api/cmd/server/main.go`
- Frontend client + runtime registration helpers
  - `apps/web/src/services/componentRegistryApi.ts`
  - `apps/web/src/services/componentRegistrar.ts`

## Backend bootstrap (Go)

```bash
go run ./apps/api/cmd/server
```

Available endpoints:

- `GET /api/component-registry`
- `GET /api/component-registry/:name`
- `POST /api/component-registry`
- `PATCH /api/component-registry/:name/enabled`

Optional settings:

- `PORT`: server bind address (default `:3000`)
- `COMPONENT_REGISTRY_PATH`: persisted file path (default `data/component-registry.json`)

## Frontend bootstrap

```ts
import { createApp } from 'vue';
import { ComponentCatalogClient } from './services/componentRegistryApi';
import { registerCatalogComponents } from './services/componentRegistrar';
import App from './App.vue';

const app = createApp(App);
const client = new ComponentCatalogClient({ baseUrl: 'http://localhost:3000/api' });

const registryPayload = await client.getEnabledComponents();
await registerCatalogComponents(app, registryPayload.components, {
  loaders: {
    DateRangePicker: () => import('advanced-inputs').then((m) => m.DateRangePicker),
    AsyncMultiSelect: () => import('advanced-inputs').then((m) => m.AsyncMultiSelect),
    InputMask: () => import('advanced-inputs').then((m) => m.InputMask),
    'pm-table': () => import('@scope/pm-table').then((m) => m.PmTable),
    DropzoneUploader: () => import('vue-dropzone').then((m) => m.DropzoneUploader),
  },
});

app.mount('#app');
```
