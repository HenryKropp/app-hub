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

The web interface runs on Replit; local Windows app execution still requires
the existing Windows agent on each user's computer. No agent binary has been
built or configured here. See `scripts/build-agent.sh`,
`docs/INFRASTRUCTURE.md`, and `AGENT_BINARY_PATH` for the existing distribution
mechanism. Replit cannot access the original machine's `/mnt/c` folders.

## Checks performed

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
