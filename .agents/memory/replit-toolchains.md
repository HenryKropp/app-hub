---
name: Replit toolchain setup
description: Non-obvious toolchain and package-registry constraints during imported-project setup.
---

Keep the imported project's Go requirement rather than downgrading it to match
Replit's available bootstrap module. Automatic toolchain downloading works here
when Go's checksum database is enabled.

**Why:** The inherited environment disabled the checksum database, which caused
the newer toolchain download to fail verification even though the requested
toolchain was available.

**How to apply:** When a future Go toolchain download fails, distinguish checksum
configuration from actual version unavailability before changing go.mod.

Package installation stalls and malformed registry tarball routes are not
evidence that application dependencies need replacement.

**Why:** Setup encountered a tarball route missing the registry's `/npm/` prefix
and lengthy automatic audit traffic. The same locked dependencies installed
successfully through the approved registry without changing the dependency set.

**How to apply:** Inspect the precise registry error and install log before
upgrading or removing packages. Do not bypass package security controls or
silently report a stalled install as successful.
