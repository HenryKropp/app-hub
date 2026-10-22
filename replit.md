# Running App Hub on Replit

The imported React/Vite frontend and Go backend retain their original structure.

## Start

Use **Run** to start the **Start application** workflow, or run:

```sh
bash scripts/run-replit.sh
```

The script builds the frontend and Go server when their sources change, then
serves both from the repository root on port 5000. The browser uses same-origin
API requests; a separate Vite server is not required. Restart the workflow after
editing sources to rebuild them.

Dependencies are installed in `frontend/node_modules`. To restore them:

```sh
npm ci --prefix frontend --no-audit --no-fund
```

Go downloads the toolchain version declared in `backend/go.mod` automatically.
The Replit modules provide Node.js 24 and the Go bootstrap toolchain; do not lower
the project's Go version to match the bootstrap version.

## Database and configuration

- This is a fresh Replit PostgreSQL instance selected by the user, not a copy of
  the database on the original developer's computer.
- Replit manages `DATABASE_URL`. Never put its value in source control.
- Development settings: `PORT=5000`, `BASE_URL` set to the HTTPS preview origin,
  and `GOSUMDB=sum.golang.org` for verified Go toolchain downloads.
- `DEV_MODE` is deliberately unset: the existing development-login shortcut is
  disabled and the Go server serves the built frontend.
- The existing backend applies its embedded migrations on startup. Database
  publishing behavior must be reviewed separately before a production launch.

Open Preview and create the first administrator account using
**Crie a conta de administrador**. No account or password has been pre-created.
Finish this step before sharing access to this fresh instance.

## Windows agent

The web interface runs on Replit; local Windows app execution requires the agent
on each user's computer. The run script builds the Windows agent for the current
development origin when sources change. `AGENT_BINARY_PATH` points to
`dist/agent/AppHubAgente.exe`, served through the authenticated download route.
Replit cannot access the original machine's `/mnt/c` folders; browser-imported
scripts instead have their source bundles stored in PostgreSQL and sent to the
agent. See `docs/SCRIPTS.md` for onboarding, manifest input mapping, supported
runtimes, dependencies, trust boundaries, and current verification limits.

## Initial import checks

- Frontend production build and Go server build succeeded.
- Preview rendered the first-administrator setup form.
- `/up` returned 200; `/api/auth/status` reported setup required; unauthenticated
  `/api/apps` returned 401.
- Frontend suite: 163 passed, one 15-second timeout in
  `frontend/src/components/ficha/AccessSection.test.tsx`.
- npm reports that several test dependencies require a newer Node.js patch
  than the currently supplied Node.js 24.13.0 module.
- The signed-in interface has not been visually verified; the initial
  administrator setup was left for the user.

Generated binaries are kept in ignored `generated-artifacts/`; frontend build
output stays in the project's existing ignored `frontend/dist/`.

## Onboarding and scripts

- Open **Começar** (`/comecar`) for setup progress.
- **Registrar apps** now supports importing Python/JavaScript sources directly
  from the browser, alongside the existing folder-scanning flow.
- The script run form is generated from the manifest. The agent is version
  0.3.0; use **Configurações > Meu computador** to install/update it.
- New-flow backend tests, Python/Node execution tests on Linux, 81 selected
  frontend tests, TypeScript, lint, the production build, and Windows agent
  compilation passed.
- The full backend suite hit Go's global 10-minute package timeout before
  finishing; it did not report an individual assertion failure before timeout.
  Do not describe the full suite as passing.
- Protected screens were tested automatically, not visually with a signed-in
  browser. A real Windows end-to-end run remains unverified.
- See `docs/SCRIPTS.md` for limits and security boundaries.
