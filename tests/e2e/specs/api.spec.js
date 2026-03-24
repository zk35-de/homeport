// @ts-check
const { test, expect } = require('@playwright/test');

test.describe('API Endpoints', () => {
  test('GET /api/health returns ok', async ({ request }) => {
    const resp = await request.get('/api/health');
    expect(resp.ok()).toBeTruthy();
    const body = await resp.json();
    expect(body.status).toBe('ok');
  });

  test('GET /api/search returns array or null for unknown term', async ({ request }) => {
    const resp = await request.get('/api/search?q=haus');
    expect(resp.ok()).toBeTruthy();
    const text = await resp.text();
    // Go json.Encode(nil) returns "null\n" when no matches
    const body = JSON.parse(text.trim());
    expect(body === null || Array.isArray(body)).toBe(true);
  });

  test('GET /api/search with empty query returns empty array', async ({ request }) => {
    const resp = await request.get('/api/search?q=');
    expect(resp.ok()).toBeTruthy();
    const body = await resp.json();
    expect(Array.isArray(body)).toBe(true);
    expect(body.length).toBe(0);
  });

  test('GET /api/search finds known service', async ({ request }) => {
    const resp = await request.get('/api/search?q=nginx');
    expect(resp.ok()).toBeTruthy();
    const body = await resp.json();
    // NGINX service exists in the DB
    const names = body.map((s) => s.name.toLowerCase());
    expect(names.some((n) => n.includes('nginx'))).toBe(true);
  });

  test('GET /api/favicon returns non-500 for valid URL', async ({ request }) => {
    const resp = await request.get('/api/favicon?url=https://example.com');
    expect(resp.status()).toBeLessThan(500);
  });

  test('404 page renders homeport 404', async ({ page }) => {
    await page.goto('/does-not-exist-xyz');
    const body = await page.locator('body').innerText();
    expect(body).toMatch(/404|nicht gefunden|not found/i);
  });
});
