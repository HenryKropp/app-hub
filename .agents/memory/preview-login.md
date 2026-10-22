---
name: Login from Preview
description: User-confirmed workaround for an authentication connection error in the embedded preview.
---

If login or initial account setup reports “Não consegui falar com o Hub” inside
Preview, check the backend and then try the app's current development URL in a
separate browser tab before changing authentication code.

**Why:** The server's status endpoint and POST validation responded correctly
while the user saw this error. The user explicitly confirmed they could enter
successfully in a separate tab. No authentication code changes were necessary.
The precise cause of the embedded-preview failure was not established.

**How to apply:** Treat a separate tab as a confirmed workaround, not proof of
a cookie or server defect. If the issue also occurs outside Preview, investigate
the new evidence rather than assuming it has the same cause.
