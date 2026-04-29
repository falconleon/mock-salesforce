// Wave 3 happy-path Playwright spec — T8 (playground), T9 (settings),
// T13 (settings/users + per-user tokens).
//
// This file is checked in for future use; running it is optional and
// no Node build pipeline is wired up. To run:
//   npm i -D @playwright/test && npx playwright install chromium
//   BASE_URL=http://localhost:8090 npx playwright test internal/server/e2e/wave3.spec.ts
//
// Boot the mock first:
//   ADMIN_TOKEN=test-admin-token go run ./cmd/salesforce-mock/ -port 8090
//
// The spec asserts via DOM/HTTP only — no screenshots required by CI.

import { test, expect, request } from '@playwright/test';

const BASE = process.env.BASE_URL ?? 'http://localhost:8090';
const USER = 'demo@falcon.local';
const PASS = 'demo123';

async function login(page) {
  await page.goto(`${BASE}/login`);
  await page.fill('input[name="email"]', USER);
  await page.fill('input[name="password"]', PASS);
  await Promise.all([
    page.waitForURL(`${BASE}/home`),
    page.click('button[type="submit"]'),
  ]);
}

test.describe('Wave 3 — Playground / Settings / Users', () => {
  test('T8 playground renders and runs SOQL', async ({ page }) => {
    await login(page);
    await page.click('nav.nav a[href="/playground"]');
    await expect(page).toHaveURL(`${BASE}/playground`);
    await expect(page.locator('button:has-text("Run")')).toBeVisible();

    // Three example chips per DoD.
    const chips = page.locator('.playground-chips a, .playground-chips button, [onclick*="sfPlaygroundLoad"]');
    expect(await chips.count()).toBeGreaterThanOrEqual(3);

    await page.fill('textarea[name="q"], input[name="q"]', 'SELECT Id, Name FROM Account LIMIT 3');
    await page.click('button:has-text("Run")');
    await expect(page.locator('.playground-meta')).toContainText('3 rows');
    await expect(page.locator('.sf-table thead th')).toHaveCount(2);

    // Errors render inline (no navigation).
    await page.fill('textarea[name="q"], input[name="q"]', 'NOT VALID SOQL');
    await page.click('button:has-text("Run")');
    await expect(page.locator('.playground-error')).toContainText('MALFORMED_QUERY');
    await expect(page).toHaveURL(`${BASE}/playground`);
  });

  test('T9 settings shows masked secret + eyeball toggle works', async ({ page }) => {
    await login(page);
    await page.click('nav.nav a[href="/settings"]');
    await expect(page).toHaveURL(`${BASE}/settings`);

    // Client ID always visible, secret masked initially.
    await expect(page.locator('text=mock-client-id')).toBeVisible();
    const secretCell = page.locator('#client-secret-cell');
    await expect(secretCell).toContainText('•');

    // OAuth token URL + cross-link to /settings/users.
    await expect(page.locator('text=/services/oauth2/token')).toBeVisible();
    await expect(page.locator('a[href="/settings/users"]')).toBeVisible();

    // Click the eyeball — HTMX swaps in plaintext.
    await page.click('#client-secret-cell .secret-toggle');
    await expect(page.locator('#client-secret-cell')).toContainText('mock-client-secret');

    // Click again — swap back to masked.
    await page.click('#client-secret-cell .secret-toggle');
    await expect(page.locator('#client-secret-cell')).toContainText('•');
  });

  test('T13 user CRUD + token mint/bearer/revoke round-trip', async ({ page, baseURL }) => {
    await login(page);
    await page.goto(`${BASE}/settings/users`);

    // Create.
    await page.fill('input[name="username"]', 'pwuser@falcon.local');
    await page.fill('input[name="name"]', 'PW User');
    await page.fill('input[name="email"]', 'pwuser@falcon.local');
    await page.fill('input[name="password"]', 'pwpass123');
    await page.click('button:has-text("Add User"), button[type="submit"]');
    await expect(page.locator('text=pwuser@falcon.local')).toBeVisible();

    // Drill into detail page.
    await page.click('a:has-text("pwuser@falcon.local")');
    await expect(page.locator('h1')).toContainText('pwuser@falcon.local');

    // Mint a token — plaintext shown exactly once.
    await page.fill('input[name="label"]', 'pw-token');
    await page.click('button:has-text("Mint Token")');
    const banner = page.locator('text=Token Minted — Copy Now').locator('..');
    await expect(banner).toBeVisible();
    const plaintext = await banner.locator('pre code').innerText();
    expect(plaintext).toMatch(/^00Dxx[0-9A-Za-z!_]+/);

    // Use the bearer against /services/data — must succeed.
    const api = await request.newContext();
    const ok = await api.get(`${BASE}/services/data/v66.0/query?q=SELECT+Id+FROM+Account+LIMIT+1`,
      { headers: { Authorization: `Bearer ${plaintext}` } });
    expect(ok.status()).toBe(200);
    expect(await ok.text()).toContain('"totalSize"');

    // Reload — plaintext token must NOT reappear.
    await page.reload();
    await expect(page.locator('body')).not.toContainText(plaintext);
    await expect(page.locator('table.sf-table')).toContainText('pw-token');

    // Revoke via the row form.
    await page.click('button:has-text("Revoke")');
    await expect(page.locator('text=No tokens yet')).toBeVisible();

    // Bearer after revoke — 401 + SF wire-format body.
    const denied = await api.get(`${BASE}/services/data/v66.0/query?q=SELECT+Id+FROM+Account+LIMIT+1`,
      { headers: { Authorization: `Bearer ${plaintext}` } });
    expect(denied.status()).toBe(401);
    expect(await denied.text()).toContain('INVALID_SESSION_ID');

    // Delete user — danger zone.
    page.once('dialog', d => d.accept());
    await page.click('button:has-text("Delete User")');
    await expect(page).toHaveURL(`${BASE}/settings/users`);
    await expect(page.locator('body')).not.toContainText('pwuser@falcon.local');
  });
});
