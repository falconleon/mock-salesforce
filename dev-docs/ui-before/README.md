# UI Before-State Screenshots

Captured at 1280×800 viewport using Playwright against the mock server on port 8081.
Credentials: `demo@falcon.local` / `demo123`.

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
