---
name: Windows agent and private previews
description: Distinguish Replit preview access restrictions from Hub pairing failures during Windows testing.
---

A Windows agent cannot inherit the browser's Replit login. Check the external HTTP response before diagnosing a pairing refusal as a Hub authorization bug.

**Why:** During a guided Windows test, the agent logged pairing HTTP 403 while a Windows curl request to the preview returned HTTP 307 to Replit's access gate (`/__replshield`). The same invalid request from the workspace reached the Hub and returned HTTP 400. Browser access alone did not establish external agent access.

**How to apply:** Compare unauthenticated external and workspace responses. Do not delete agent credentials, disable antivirus, or weaken Hub authentication to resolve a platform access gate. Explain exposure and obtain user agreement before making a development URL public; consult current Replit documentation for access options. This evidence does not establish that pairing succeeds once the gate is resolved.
