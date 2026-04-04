# Back to FalconMode Button — Design Spec

**Date:** 2026-04-03
**Status:** Approved

## Problem

When users navigate from FalconMode to the Salesforce mock, there's no way to
return to where they were. They must manually navigate back to FalconMode.

## Solution

FalconMode passes a `falcon_return` query parameter on all Salesforce mock
links. The mock captures this URL, persists it in a cookie, and renders a
"Back to FalconMode" button in the header.

## Flow

1. FalconMode links: `https://sf-mock.../lightning/r/Case/{id}/view?falcon_return=<encoded_url>`
2. Auth middleware redirects unauthenticated users to `/?falcon_return=...` (preserving the param)
3. Login form includes `<input type="hidden" name="falcon_return" value="...">`
4. `POST /login` reads `falcon_return`, sets `falcon_return` cookie, redirects to case list
5. `layout.html` renders the button server-side if `falcon_return` cookie is present
6. Clicking the button navigates to the stored URL and clears the cookie

## Security

Validate `falcon_return` URL against an allowed-origins list to prevent open
redirects. Default allowed patterns: `*.orb.local`, `localhost`, `127.0.0.1`.
Configurable via `FALCON_RETURN_ALLOWED_ORIGINS` env var.

## Files Changed

| File | Change |
|------|--------|
| `internal/server/middleware/auth.go` | Preserve `falcon_return` through login redirects; set/read cookie in LoginHandler |
| `internal/server/middleware/falcon_return.go` | New: URL validation, cookie helpers |
| `internal/server/templates/login.html` | Hidden form field for `falcon_return` |
| `internal/server/templates/layout.html` | "Back to FalconMode" button in header |
| `internal/server/static/salesforce.css` | Button styling |
| `internal/server/ui_handlers.go` | Pass `FalconReturn` to template data from cookie |

## Cookie Spec

- Name: `falcon_return`
- Value: raw URL (not encoded — already validated)
- HttpOnly: false (readable by JS for clear-on-click, but also rendered server-side)
- SameSite: Lax
- MaxAge: 28800 (8 hours, matches session cookie)
- Path: /

## Button Placement

Right side of the header nav bar. Outlined/lighter style to distinguish from
Salesforce nav items. Includes a left arrow icon.

## FalconMode Developer Request

All links to the Salesforce mock must include `?falcon_return=<url_encode(window.location.href)>`.
