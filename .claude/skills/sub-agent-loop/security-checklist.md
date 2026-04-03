# Security Checklist for Implementation Agents

This checklist is mandatory for every implementation task. It captures recurring
vulnerability patterns identified across 5 rounds of security audits (Session 70).

## MANDATORY: Check These Before Committing

### 1. RBAC on ALL Handlers (Recurring — found in every audit round)

Every HTTP handler in the `web/` layer MUST have a `getPagePermissions` + `CanAccess`
check. This includes:

- **Main CRUD handlers** (Create, Read, Update, Delete)
- **Validation/utility endpoints** (`Validate*`, `Check*`, `Test*`, `Apply*`)
- **Detail/tab handlers** that fetch and display data
- **Export handlers** that serve files

**The recurring mistake:** Sub-agents add RBAC to the main handlers but skip the
"helper" endpoints in the same file. Every audit round found 1-3 more of these.

**How to verify:** After implementing, grep the handler file for ALL `func (h *Handlers)`
methods and confirm each one either has a `getPagePermissions` call or is documented
as intentionally public.

### 2. Error Message Sanitization

Never return raw `err.Error()` to API clients or render it in HTML. This leaks:
- SQL constraint names and table structures
- Internal hostnames and IP addresses
- API key fragments from provider errors
- Go type names from JSON decode errors

**Pattern:**
```go
// WRONG:
WriteError(w, ErrValidation, "invalid request body: "+err.Error())
http.Error(w, err.Error(), http.StatusInternalServerError)
data.FormError = err.Error()

// RIGHT:
log.Printf("[ERROR] handler: %v", err)
WriteError(w, ErrValidation, "invalid request body")
http.Error(w, "internal error", http.StatusInternalServerError)
data.FormError = "Operation failed. Please try again."
```

Also: never reflect user-supplied input (names, IDs) in error messages:
```go
// WRONG: WriteError(w, ErrNotFound, "model not found: "+name)
// RIGHT: WriteError(w, ErrNotFound, "model not found")
```

### 3. URL Validation — Always Parse, Never String-Match

When validating URLs (SSRF allowlists, redirect validation, etc.):
- Always use `url.Parse` and compare against `parsed.Hostname()`
- Never use `strings.Contains`, `strings.HasPrefix` on raw URL strings
- For wildcard domains: `host == suffix || strings.HasSuffix(host, "."+suffix)`
- For redirects: verify `u.Host == ""` AND reject `//` prefix

### 4. Proxy Header Trust

`X-Real-IP`, `X-Forwarded-For`, and `X-Forwarded-User` are attacker-controlled
unless the request comes from a trusted proxy. Always gate on `isTrustedProxy()`:

```go
// WRONG:
ip := r.Header.Get("X-Real-IP")

// RIGHT:
ip, _, _ := strings.Cut(r.RemoteAddr, ":")
if isTrustedProxy(r.RemoteAddr) {
    if v := r.Header.Get("X-Real-IP"); v != "" { ip = v }
}
```

### 5. HTTP Client Redirect Safety

Go's `http.Client` follows redirects by default. An allowed URL can redirect to
a private IP. Always add `CheckRedirect` that validates each hop:

```go
client := &http.Client{
    Timeout: 30 * time.Second,
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        if err := validateURL(req.URL.String()); err != nil {
            return fmt.Errorf("redirect blocked: %w", err)
        }
        if len(via) >= 10 { return fmt.Errorf("too many redirects") }
        return nil
    },
}
```

### 6. Content-Disposition Header Safety

User-supplied values in `Content-Disposition` filenames can inject header parameters.
`template.HTMLEscapeString` is wrong context — it's an HTTP header, not HTML.

```go
// WRONG:
filename := fmt.Sprintf("export-%s.csv", userInput)
// WRONG:
filename := template.HTMLEscapeString(userInput)

// RIGHT:
safeInput := regexp.MustCompile(`[^a-zA-Z0-9\-_]`).ReplaceAllString(userInput, "")
w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, safeInput))
```

### 7. Cookie Attribute Consistency

Set and clear operations MUST use identical attributes (`SameSite`, `Path`, `Secure`).
Mismatched attributes cause browsers to silently ignore the clear operation.

### 8. Token/Claims Type Field

When creating `Claims` from any source (JWT, session, context enrichment), always
set `Type: "access"` explicitly. An empty `Type` field causes downstream
`claims.Type == "access"` checks to silently fail.

### 9. Body Size Limits

`http.MaxBytesReader` must be the FIRST thing applied in a handler, before any
session lookups, RBAC checks, or other work. Apply it before `ParseForm`.

### 10. Feature Additions Create Attack Surface

When a fix adds a new feature (flag, endpoint, CLI option), evaluate whether it
introduces a new vulnerability:
- CLI flags with secrets → visible in `ps aux` and shell history
- New endpoints → need RBAC registration
- New env var checks → must match codebase convention (`== "true"`)

### 11. Fix ALL Instances, Not Just the One Mentioned

When fixing a pattern (missing RBAC, raw error, etc.), grep for ALL instances
of the same pattern across the entire codebase. The task may mention one file,
but the same bug likely exists in sibling handlers.

### 12. `DEV_MODE` Convention

Always use `os.Getenv("DEV_MODE") == "true"`. Never `!= ""`.
`DEV_MODE=false` must NOT activate dev-mode behavior.
