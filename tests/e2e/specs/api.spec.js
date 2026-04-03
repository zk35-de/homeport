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

  test('GET /api/search returns valid structure', async ({ request }) => {
    // Search with a term that may or may not match — just verify response shape
    const resp = await request.get('/api/search?q=home');
    expect(resp.ok()).toBeTruthy();
    const text = await resp.text();
    const body = JSON.parse(text.trim());
    // null (no results) or array of objects with name/url
    if (body !== null) {
      expect(Array.isArray(body)).toBe(true);
      if (body.length > 0) {
        expect(body[0]).toHaveProperty('name');
        expect(body[0]).toHaveProperty('url');
      }
    }
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
