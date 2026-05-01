# UI After-State Screenshots (v2 — FalconMode Brand Palette)

Captured at 1280×800 viewport using Playwright against the mock server on port 9090.
Credentials: `demo@falcon.local` / `demo123`.

**v2 changes applied (UI-5):** FalconMode brand palette retune (deep-blue primary `#1A3A6D`, slate-blue muted, charcoal text, light-gray bg), breadcrumb link underline (WCAG 1.4.1), heading order h3→h2 (WCAG 1.3.1), removed dead `.playground-chips` CSS rules.

**v3 changes applied (UI-7):** ESCALATED status pill now uses FalconMode amber (`--color-warning-bg: #FDF4E6` / `--color-warning-text: #7C2D12`) via `.badge-orange` instead of legacy red. P1 priority pills also use the same amber tones (consequential — shared `.badge-orange` class). Recaptured `cases-list.png` and `account-detail.png` (ESCALATED rows visible in both).

| filename | URL | description |
|---|---|---|
| login.png | /login | Login page (logged-out state) |
| home.png | /home | Home page — customer (Account) list with open case counts |
| accounts-list.png | /lightning/o/Account/list | Accounts list view |
| account-detail.png | /lightning/r/Account/0013t00002AbCdEAAV/view | Account detail — Acme Corporation |
| cases-list.png | /lightning/o/Case/list | Cases list view |
| case-tab-emails.png | /lightning/r/Case/5003t00002AbCdEAAV/view?tab=emails | Case detail — Emails tab (case 00123456) |
| case-tab-comments.png | /lightning/r/Case/5003t00002AbCdEAAV/view?tab=comments | Case detail — Comments tab |
| case-tab-feed.png | /lightning/r/Case/5003t00002AbCdEAAV/view?tab=feed | Case detail — Feed tab |
| case-tab-activities.png | /lightning/r/Case/5003t00002AbCdEAAV/view?tab=activities | Case detail — Activities tab |
| case-tab-files.png | /lightning/r/Case/5003t00002AbCdEAAV/view?tab=files | Case detail — Files tab |
| contact-detail.png | /lightning/r/Contact/0033t00002CdEfGAAV/view | Contact detail — John Smith |
| settings.png | /settings | Settings page — OAuth client credentials |
| settings-users.png | /settings/users | Settings — Users management |
| playground.png | /playground | SOQL Playground |
| oauth-authorize.png | /services/oauth2/authorize?client_id=mock-client-id&response_type=code&redirect_uri=http://localhost:1717/OauthRedirect&code_challenge=...&code_challenge_method=S256&state=abc | OAuth authorization consent page |
