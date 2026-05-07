# Realtime Prototype - Inicio rápido

## Requisitos

- Node.js `20.19+` (o `22+`)
- Go `1.22+`

## 1) Variables de entorno

El proyecto usa estos archivos de ejemplo:

- `apps/api/.env.example`
- `apps/web/.env.example`

Desde la raíz del repo:

```bash
cd apps/api
cp .env.example .env
cd ../web
cp .env.example .env
```

Después completa al menos:

- `apps/api/.env`
  - `CEREBRAS_API_KEY` (obligatoria para generación)
  - `CEREBRAS_API_URL` (por defecto)
  - `CEREBRAS_MODEL` (por defecto)
  - `CEREBRAS_REASONING_EFFORT` (opcional)
  - `CEREBRAS_TEMPERATURE` (opcional)
  - `PORT` (por defecto `3000`)
  - `CORS_ALLOWED_ORIGIN` (por defecto `http://localhost:5173`)

- `apps/web/.env`
  - `VITE_API_BASE_URL` (por defecto `http://localhost:3000/api`)

## 2) Backend (Go)

En una terminal nueva:

```bash
cd apps/api
go mod download
go run ./cmd/server
```

- El backend queda en `http://localhost:3000`
- La API principal queda en `http://localhost:3000/api`

## 3) Frontend (Vite + Vue)

En otra terminal:

```bash
cd apps/web
bun install    # o npm install
bun run dev    # o npm run dev
```

- Abrir en el navegador: `http://localhost:5173`

## 4) Storybook

Para componentes del catálogo:

```bash
cd apps/web
bun run storybook   # o npm run storybook
```

- Abrir en el navegador: `http://localhost:6006`

## 5) Orden recomendado de arranque

1. Levantar primero el backend.
2. Levantar el frontend.
3. Abrir `http://localhost:5173`.
4. Levantar Storybook si lo necesitas.
