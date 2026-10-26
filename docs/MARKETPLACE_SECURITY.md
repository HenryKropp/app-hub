# Marketplace B2B: boundaries, lifecycle and release gates

## Current scope (internal, trusted code only)

The first company remains at `/`. Other companies use `/w/<slug>/` and have
separate Postgres schemas, sessions, handlers, roles, apps, agent connections
and histories. The address chooses a company; users must still sign in there.
Creating a company requires `manage_roles` and re-entering the **current**
administrator password. The new company starts with a separate local account
with that administrator's name, email and existing password hash. After that,
credentials and membership evolve independently. There is no shared identity
provider, company invitation or platform-wide directory yet. Do not confuse
an admin of one company with a platform operator.

An administrator can review internal apps in **Segurança**: record purpose,
data classification, informational outbound-domain declarations, approve
current manifest/version, suspend an app, or disable local execution globally.
New apps are unapproved by default. A change to the manifest or a version
change detected by the version scanner invalidates the approval. Suspension
blocks new runs and cancels pending jobs. It cannot stop an agent already
running code, or an offline computer that already accepted a job.

Browser-imported scripts execute the copy stored in the company's database,
not an arbitrary working-folder edit. If that folder diverges from the stored
bundle (including after code restoration), execution is refused and the
author must reimport the desired source. Code restoration does not secretly
replace the durable import bundle.

The portal blocks server-side folder scanning/registration and offers browser
script import. Tenant Windows agents download a ZIP: extract the EXE and
`apphub-url.txt` together before launching; the URL is public routing metadata,
**not a secret**. The original company's existing EXE remains supported.
Agents store their state per workspace. Never run downloaded unknown scripts
just because the Hub can launch them; there is no host/network sandbox.

Incoming source archives reject credential-file names and, by default,
possible embedded secrets even if the user asks to confirm. Detectors can
miss secrets; this is not a credential vault. A manager may relax the import
block but that does not make a secret-containing package safe to distribute.

Mutation attempts and outcomes are recorded with actor, server route template,
time and status, without body/cookie/credentials. The latest 100 records and
limited metadata can be exported. Audit isn't tamper-proof or transactionally
coupled to all business operations. Configured run retention requires an
**explicit** manual cleanup; it deletes old finished run records/jobs via
database cascades. It does not remove Windows files, source bundles, backups
or audit history. It is not automated legal deletion.

## Target distribution lifecycle (not implemented yet)

| Stage | Required review and evidence |
| --- | --- |
| Author creates | Package with source provenance, declared data/permissions, reproducible dependencies, no embedded secrets. |
| Submit | Immutable version digest, license/ownership and publisher identity; scan and non-self approval. |
| Publish | Marketplace listing is visible without exposing customer secrets or code by default; explicit external policy review. |
| Company adopts | Admin chooses exact version, purpose and legal basis, approves connectors/data scopes and grants employee access. |
| Configure | Credentials supplied by **that company** to an encrypted vault with independently managed key and scoped retrieval; no value in manifest or retained job. |
| Execute | Trusted package with a defined isolation boundary, enforced network/filesystem policy and agent/sandbox choice. Record reason, actor and outcome. |
| Update | Separate review for new version, changed permissions and dependencies; rollback to known digest. Never silently replace a deployed version. |
| Suspend | Halt adoption/new jobs, notify affected admins, rotate affected credentials; already-running/offline local work needs separate incident response. |
| Remove | Defined window for run/input/result deletion, backup expiration, audit retention, data portability and processor termination. |

## Release gates

**Do not enable marketplace publication or external install yet.** Required
work includes: vault with key lifecycle, strong runner isolation, signed or
content-addressed releases, reviewer separation, tests for cross-company
objects and downloads, independent security review, durable shared limits,
incident handling, and legal validation of LGPD controller/operator roles,
processors, notices, contracts, subject rights and retention. Server-side
domains are **declared, not enforced**. The present portal is a foundation
for controlled internal pilots, not a compliance certification.
