# Running Ziv on Replit

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

## Post-merge setup

`scripts/post-merge.sh` restores locked frontend dependencies, downloads Go
modules, and builds the frontend, Windows agent, and server. It exits after
preparation; Replit's workflow reconciliation restarts the running application.
Embedded database migrations remain part of normal server startup. The hook is
configured with a five-minute timeout.

## Onboarding and scripts

- Open **Começar** (`/comecar`) for setup progress.
- **Registrar apps** now supports importing Python/JavaScript sources directly
  from the browser, alongside the existing folder-scanning flow.
- The script run form is generated from the manifest. The agent is version
  0.3.2; use **Configurações > Meu computador** to install/update it.
- New-flow backend tests, Python/Node execution tests on Linux, 81 selected
  frontend tests, TypeScript, lint, the production build, and Windows agent
  compilation passed.
- The full backend suite hit Go's global 10-minute package timeout before
  finishing; it did not report an individual assertion failure before timeout.
  Do not describe the full suite as passing.
- Protected screens were tested automatically, not visually with a signed-in
  browser. A real Windows end-to-end run remains unverified.
- See `docs/SCRIPTS.md` for limits and security boundaries.

## Multi-company security pilot

The original company remains at `/`; administrators can create isolated
company workspaces under `/w/<slug>/` in **Segurança**. Their data uses separate
PostgreSQL schemas and company-scoped sessions. `docs/MARKETPLACE_SECURITY.md`
describes what is operational today and what is still blocked. The project's
security reference is `threat_model.md`. No external marketplace is enabled.
The credential vault stays locked without its independent workspace Secret.
Browser imports replace server filesystem scanning in this
portal; existing apps remain in the original company's database namespace.

## GitHub shared-app integration

The GitHub shared-app implementation is integrated alongside the multi-company
portal: shared manifests, frontend sections, server page, vault/backup libraries,
and embedded migrations are present. API requests, download links, and login
handoffs retain the selected workspace prefix.

The executable entry point intentionally continues to use `workspace.Portal`.
The upstream Podman supervisor/gateway is **not connected** in this portal.
It assumes globally named app IDs, databases, containers and hostnames; simply
attaching it to each workspace would not isolate these resources. Shared-app
actions therefore return an explicit `shared_runtime_unavailable` error (503).
This applies even if a vault key is configured. Do not replace this with a fake
engine or weaken company isolation to enable a demo.

Local Windows-agent scripts, approvals, auditing, retention and company creation
continue to work as before. Running hosted shared apps requires a compatible
execution server and a workspace-aware resource/gateway design first. The
upstream README describes that single-company hosting deployment, not a fully
enabled runtime in this Replit portal.

The new history migration removes `.env`-style secret files from stored versions
and deletes unreferenced code blobs. It does not delete companies or accounts.
Pre-update database backups are restricted local operational files, not public
download assets; they may contain sensitive historical data and must not be
committed or published.

## Ziv identity and agent updates

The GitHub Ziv identity and agent 0.3.2 are integrated without replacing the
multi-company server. Downloads use `ZivAgente.exe` (or a company-specific ZIP
containing that executable and `apphub-url.txt`). Installed agent filenames,
startup settings, and the sidecar retain their legacy technical names for
compatibility with existing computers.

The new stop action and simultaneous-app support remain company-scoped. Stopped
executions participate in the company's existing manual retention policy, and
cancelling a pending run removes its script payload. The new stop migration
coexists with session hashing because migrations are tracked by full filename,
not just the numeric prefix.

Upstream adoption/renaming of hosted containers, database containers and shared
storage is intentionally not run by this portal: hosted execution is still
disabled until runtime resources and gateway access are isolated per company.

## Company entry and membership

Anonymous visits to `/` open `/boas-vindas`; existing company sessions still open
the catalog. `/entrar` remains the original company's explicit login. Visitors
can create a separate company with its first administrator, or enter an existing
company using its identifier/URL. No public company directory is exposed.
Public company registration shares the portal's 20-company capacity limit.

The public presentation at `/boas-vindas` does not contain login or registration
forms. **Login / Cadastre-se** opens `/acesso` for company selection; company
creation is a separate screen at `/cadastre-se`. Existing company login and
invitation links keep their workspace paths and authentication boundaries.

At a company's login, a visitor can submit a name/email access request, without
a password. In **Equipe → Acessos**, managers with `manage_roles` can approve or
reject requests, issue invitations with a chosen role, and revoke invitations.
Approval generates an invitation, not an account. Links expire after seven days,
are single-use, and carry a 256-bit bearer token in the URL fragment. Only its
SHA-256 hash is stored. Redemption creates the account and session transactionally
inside that company's schema. A replacement invite invalidates older links for
the same email in that company. Deleting a role invalidates its invitations.

There is no email provider or automatic email ownership verification. Managers
must send invitation links directly to the intended recipient's email through a
trusted channel; anyone holding the link can activate that email's account.
Public requests are unverified claims, not evidence of company membership.
Registration is rate limited, but production-wide abuse protection and verified
company ownership are not implemented. Do not describe these flows as compliance
certification or unrestricted public-marketplace readiness.

## Company credential vault

See `docs/CREDENTIAL_VAULT.md` for configuration, key rotation and restore safety.
Only `CREDENTIAL_VAULT_KEYRING` in workspace Secrets unlocks the portal's vault;
neither session nor hosted-app keys are reused. Invalid configured keyrings stop
startup. Without a key, metadata and revocation still work.

The current intermediary uses fixed HTTPS HEAD Bearer requests and returns only
status codes. Do not add generic response forwarding or agent injection:
providers can echo credentials in output/errors, and local scripts can log them.
Business connectors need purpose-specific safe adapters. Hosted execution and
external distribution remain disabled even when the vault is configured.

After restoring any database backup, keep the portal stopped and run
`scripts/quarantine-restored-credentials.sql` with `psql -v ON_ERROR_STOP=1`
before allowing traffic. This intentionally removes all restored credential
envelopes across companies: old backups cannot be trusted to contain later
revocations. Revoke/rotate upstream and register fresh credentials afterward.
