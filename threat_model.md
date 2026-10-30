# Threat Model — App Hub / B2B microapps

## Project Overview

App Hub is a Go/PostgreSQL API with a React frontend. People in a company
discover, register, review and run microapps. A Windows agent runs trusted local
programs on a person's computer. The portal uses one database with a distinct
PostgreSQL schema, connection pool, session namespace and in-memory handler per
company. The original company's data remains in `public`; new companies use
server-generated schema names. A URL `/w/{slug}` selects a company, **not an
identity**. This is logical application isolation, not separate infrastructure
or database credentials.

**Launch constraint:** There is no externally distributable marketplace,
isolated execution sandbox or verified compliance certification. Do not enable
publication of third-party executable code based on the internal review flag.

## Assets

- Password hashes, browser sessions and Windows agent tokens: grant access to
  the company's accounts and potentially execution on a person's computer.
- Source code, manifests and version history: proprietary business logic and
  potential embedded secrets. The import scanner can miss credentials.
- Input files, queued job payloads, output paths and downloaded results: may
  contain confidential business or personal data.
- Company profile, approvals and audit history: controls and evidence for
  operating apps; tampering could permit unauthorized execution.
- Credentials: company/app/environment/role-scoped Bearer tokens have a separate
  AES-256-GCM envelope vault. Master keys live only in workspace Secrets, never
  in the DB; it stays locked when unconfigured. No value-read API exists.
  Values never enter a manifest, package, job or marketplace listing. The only
  supported use adapter is a fixed HTTPS HEAD request, returning status only.
  See `docs/CREDENTIAL_VAULT.md` for key lifecycle and restore quarantine.

## Trust Boundaries

- Browser → API: session authentication and server-side role/app authorization;
  never trust a company ID, URL, claimed owner or approval supplied by a client.
- URL selector → company DB schema: the registry maps validated slugs to
  server-generated schema names; each pool sets one search_path without public
  fallback. Queries must stay on that pool. The single DB role still has access
  to all schemas; SQL injection or server compromise can defeat this boundary.
- Hub → Windows agent: token identifies a computer within one company.
  The local agent executes code with the Windows user's access. It is **not**
  a sandbox; it may read other files or send data over the network.
- Creator → approver → consumer: publishing code, approving internal use,
  distributing externally and accessing a customer's data are distinct rights.
  Only internal approval is implemented; external distribution is blocked.
- Service → filesystem / dependency ecosystem / external APIs: browsing host
  folders is disabled in the portal, but installed dependencies and approved
  local scripts may execute uncontrolled network/host operations.
- Development → published production: dev origin, compiled Windows agent URL
  and DB publishing must be reviewed independently before launch.

## Scan Anchors

- `backend/cmd/server/main.go`, `backend/internal/workspace/portal.go`: root
  entry, company routing, schema setup and connection pooling.
- `backend/internal/handler/handler.go`, `security.go`,
  `security_middleware.go`: auth, policy checks, origins, audit and limits.
- `backend/internal/handler/{agents,runs,script_import}.go` and
  `backend/internal/agent/scripts.go`: pairing, dispatch and executable inputs.
- `backend/internal/database/migrations/`: schema, migrations and retention.
- Browser and API: `frontend/src/lib/api.ts`, `frontend/src/pages/SecurityPage.tsx`.
- Tests/dev-only: `backend/internal/*/*_test.go`; do not treat DEV_MODE as
  protection in production.

## Threat Categories

### Spoofing

An attacker may guess or steal a session, agent token or company address.
Sessions use random cookies scoped to company paths and stored as SHA-256
digests. Passwords use bcrypt (new hashes cost 12); old hashes remain until
password rotation. Agent tokens are compared within the selected company's
schema. The company URL must never authenticate a person, and sessions must
never be interchangeable across schemas. Enforce TLS and add MFA for privileged
accounts before an external rollout.

### Tampering

An author could modify the manifest/code after approval. Internal approvals
are bound to a SHA-256 digest of the stored manifest and the latest recorded
version; code changes detected by the version check invalidate the approval
before a run. This is **not** a signed marketplace package: externally
distributed, immutable packages and independent verification do not exist yet.
Unclaimed jobs can be cancelled; work already handed to an offline agent cannot
be recalled.

### Repudiation

Mutating browser API calls create an audit *attempt* before the handler executes
and update it with the route template and response code afterward. No body or
cookie is logged by this audit. The second write is not atomic with the action,
so a failed audit completion can leave an attempt with no outcome. The log is
stored in the same DB as the app, not immutable or externally replicated.
Before regulated use, add tamper-resistant audit delivery and review retention
and incident procedures.

### Information disclosure

An app, run, person, download or agent token must never resolve in a different
company. Separate schemas/pools and scoped handlers enforce this at the API
layer; tests exercise overlapping IDs and wrong-company credentials. The
current DB role and infrastructure are shared. The optional metadata export
excludes code, credentials, files and execution inputs; it is **not** a full
data-subject export. Import scans block common secret patterns by default in
the portal, but cannot guarantee detection. Execution arguments, inputs and
outputs may contain sensitive data; don't disclose them in audit events.

### Denial of service

Creation is limited to 20 companies per portal and requires an existing
company administrator's password. Authentication/pairing endpoints have
in-memory per-company/IP attempt limits; a proxy can aggregate many people
under one address, and restarting clears the limits. Request uploads have
existing size caps. Before public access, replace in-memory limits with
shared rate controls and test resource exhaustion of schema pools/jobs.

### Elevation of privilege

The frontend is never an authorization boundary: security settings, app
review, audit export and creation require `manage_roles` server-side.
Approval is for **trusted internal use**, not permission to run third-party
code safely. The agent and local dependencies still inherit Windows user
permissions. New customers must not be given general shell execution or
filesystem scanning on the Hub server. Use a separate trust domain for hosted
microapps and independent execution isolation before external publishing.

## Required guarantees before external launch

1. A credential vault with separately managed encryption keys, access/revocation
   policy, and no plaintext value in source, logs or persisted job payloads:
   implemented for fixed-destination Bearer checks, but operational key
   provisioning/recovery must be completed before use. No generic script,
   database or OAuth credential delivery is enabled. Restoring an old DB without
   mandatory vault quarantine would revive old grants; do not put a raw restore
   online. Rewrap does not revoke a compromised token at its provider.
2. Immutable externally distributed versions, publisher verification,
   two-person review, emergency suspension, provenance and dependency checks.
3. Real process/network/filesystem isolation for untrusted code; informational
   declared domains must not be presented as enforced egress rules.
4. Validated multi-company isolation, backup/restore and deletion tests,
   incident response and independent security assessment.
5. Legal review of LGPD roles, data processing agreements, retention,
   international transfers, subprocessors and data-subject rights based on
   actual traffic. There is no claim of LGPD certification here.
