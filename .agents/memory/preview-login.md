---
name: Login from Preview
description: Embedded Preview navigation and a user-confirmed workaround for login connection errors.
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

For navigation inside the embedded Preview, give only a path such as
`/boas-vindas`, not a complete development URL.

**Why:** The Preview address field edits the path, not the domain. Giving the
user a complete URL resulted in a malformed `/https:/.../boas-vindas` path.
Official Preview documentation confirms path-only navigation. A `:5000` shown
in preview context alone does not establish that the embedded proxy is broken;
testing that address externally is not equivalent to testing the embedded UI.

**How to apply:** Tell the user to replace the entire editable path with the
short path and press Enter. Do not ask them to edit the Preview's domain/port or
repeat full-URL instructions when the goal is navigation within Preview.

Distinguish the editor's “This is taking longer than expected / Ask Agent to
fix the preview” placeholder from a loading/error screen rendered by the app.

**Why:** The user provided a capture of this outer Preview placeholder while
the workflow reported port 5000 open, the listener bound `0.0.0.0`, and the
development proxy returned the app successfully. Direct app screenshots did
not reproduce or verify the editor's embedded-panel state.

**How to apply:** Check workflow registration/forwarding rather than changing
login or routes. After a bounded re-registration attempt, report the remaining
editor-side uncertainty plainly. Do not claim the user's Preview is fixed just
because curl or an anonymous app screenshot succeeds.
