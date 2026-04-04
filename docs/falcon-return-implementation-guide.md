# "Back to FalconMode" Button — Implementation Guide for Mock Services

This document describes how to add a "Back to FalconMode" button to a mock
service (Jira, Salesforce, etc.) that allows users to return to the FalconMode
page they came from.

## How It Works

1. **FalconMode** adds `?falcon_return=<url-encoded-page-url>` to all links
   pointing to the mock service.
2. **The mock service** captures the `falcon_return` query parameter, validates
   it, stores it in a cookie, and displays a "Back to FalconMode" button in the
   header when the cookie is present.

## What You Need to Implement

### 1. Capture `falcon_return` on Any Incoming Request

Add middleware (or equivalent) that runs on every request:

```
if request has query param "falcon_return":
    url = validate(request.query["falcon_return"])
    if url is valid:
        set cookie "falcon_return" = url
            path=/
            httpOnly=false  (JS needs to read it to clear on click)
            sameSite=Lax
            maxAge=28800    (8 hours)
```

### 2. Preserve Through Login Flow

If your mock has a login page, the `falcon_return` param must survive the
redirect chain:

- **Auth middleware redirect**: When redirecting to login, append the param:
  `/?falcon_return=<url-encoded-value>`
- **Login form**: Include a hidden field:
  `<input type="hidden" name="falcon_return" value="...">`
- **Login POST handler**: Read the hidden field, validate, and set the cookie
  before redirecting to the post-login page.
- **Failed login redirect**: Preserve the param:
  `/?error=invalid&falcon_return=<url-encoded-value>`

### 3. Validate the Return URL

**Critical for security** — prevents open-redirect attacks.

Accept only URLs where:
- Scheme is `http` or `https`
- Host matches an allowed pattern

Default allowed patterns:
- `*.orb.local` (OrbStack dev domains)
- `localhost`
- `127.0.0.1`

Optionally configurable via env var (e.g., `FALCON_RETURN_ALLOWED_ORIGINS`).

Reject everything else silently (treat as if the param wasn't provided).

### 4. Render the Button

In your page layout/header, add a button that:
- Is hidden by default (`display: none`)
- On page load, a small script reads the `falcon_return` cookie
- If the cookie exists, shows the button with `href` set to the cookie value
- On click, clears the cookie (`max-age=0`)

**HTML (in header, after nav):**
```html
<a id="falcon-return-btn" class="falcon-return-btn" style="display:none" href="#">
    <!-- left arrow icon -->
    Back to FalconMode
</a>
```

**JavaScript (end of body):**
```js
(function() {
    var m = document.cookie.match(/(?:^|;\s*)falcon_return=([^;]+)/);
    if (!m) return;
    var url = decodeURIComponent(m[1]);
    var btn = document.getElementById('falcon-return-btn');
    btn.href = url;
    btn.style.display = '';
    btn.addEventListener('click', function() {
        document.cookie = 'falcon_return=; path=/; max-age=0';
    });
})();
```

**CSS:**
```css
.falcon-return-btn {
    margin-left: auto;
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    color: white;
    text-decoration: none;
    font-size: 0.8125rem;
    font-weight: 500;
    padding: 0.25rem 0.75rem;
    border: 1px solid rgba(255,255,255,0.4);
    border-radius: 4px;
    white-space: nowrap;
}
.falcon-return-btn:hover {
    background: rgba(255,255,255,0.15);
    border-color: rgba(255,255,255,0.7);
}
```

Adjust colors to match your mock's header theme.

## What FalconMode Needs to Do

All links from FalconMode to the mock service must include the `falcon_return`
query parameter with the current page URL, URL-encoded:

```
Original:  https://jira-mock.../browse/PROJ-123
With param: https://jira-mock.../browse/PROJ-123?falcon_return=https%3A%2F%2Fnginx.falcon-backend.orb.local%2Fprocessing%2Fcases%2F456
```

The value should be `encodeURIComponent(window.location.href)` at the time the
user clicks the link.

## Reference Implementation

See the mock-salesforce repo for a working Go implementation:
- `internal/server/middleware/falcon_return.go` — validation, cookie helpers, capture middleware
- `internal/server/middleware/falcon_return_test.go` — validation tests
- `internal/server/middleware/auth.go` — login flow preservation
- `internal/server/templates/layout.html` — button HTML + JS
- `internal/server/static/salesforce.css` — button styling
