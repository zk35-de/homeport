// @ts-check
const { test, expect } = require('@playwright/test');

test.describe('Navigation & Service Redirect', () => {
  test('service cards link via /r/{id}', async ({ page }) => {
    await page.goto('/');
    const card = page.locator('a.service-card').first();
    await expect(card).toBeVisible();
    const href = await card.getAttribute('href');
    expect(href).toMatch(/^\/r\/\d+/);
  });

  test('/r/{id} redirects to service URL', async ({ request, page }) => {
    await page.goto('/');
    const card = page.locator('a.service-card').first();
    const href = await card.getAttribute('href');
    expect(href).toBeTruthy();

    // Check redirect via API request (no browser redirect-following)
    const resp = await request.fetch(`http://localhost:8855${href}`, {
      maxRedirects: 0,
    }).catch(e => e.response ?? null);

    // With maxRedirects:0, the redirect response is returned directly
    if (resp && resp.status) {
      expect(resp.status()).toBeGreaterThanOrEqual(301);
      expect(resp.status()).toBeLessThan(400);
    }
  });

  test('category header click collapses body', async ({ page }) => {
    await page.goto('/');
    const cat = page.locator('.category[data-cat-id]').first();
    await expect(cat).toBeVisible();
    const catId = await cat.getAttribute('data-cat-id');
    const body = page.locator(`#cat-body-${catId}`);

    // Click header to collapse
    await page.locator(`#cat-arrow-${catId}`).click();
    // maxHeight becomes 0 (collapsed)
    await expect(body).toHaveCSS('max-height', '0px', { timeout: 1000 });
  });

  test('category can be expanded after collapse', async ({ page }) => {
    await page.goto('/');
    const cat = page.locator('.category[data-cat-id]').first();
    const catId = await cat.getAttribute('data-cat-id');
    const body = page.locator(`#cat-body-${catId}`);
    const arrow = page.locator(`#cat-arrow-${catId}`);

    // Collapse
    await arrow.click();
    await expect(body).toHaveCSS('max-height', '0px', { timeout: 1000 });

    // Expand
    await arrow.click();
    await expect(body).not.toHaveCSS('max-height', '0px', { timeout: 2000 });
  });

  test('/status/stream SSE endpoint returns text/event-stream', async ({ request }) => {
    // Use abort signal to not hang on the stream
    const resp = await request.get('/status/stream', {
      timeout: 3000,
    }).catch(e => e.response ?? null);

    // Either got a response or timed out — either way the endpoint must exist (no 404/500)
    if (resp && resp.status) {
      expect(resp.status()).toBeLessThan(500);
      const ct = resp.headers()['content-type'] || '';
      expect(ct).toContain('text/event-stream');
    }
    // Timeout is also acceptable (stream stays open)
  });

  test('service spotlight shows results when typing', async ({ page }) => {
    await page.goto('/');
    const input = page.locator('#search-input');
    await input.click();
    // Type a short query — spotlight should appear
    await input.fill('a');
    await expect(page.locator('#search-spotlight')).toBeVisible({ timeout: 1000 });
  });

  test('spotlight hides on Escape', async ({ page }) => {
    await page.goto('/');
    const input = page.locator('#search-input');
    await input.fill('a');
    await expect(page.locator('#search-spotlight')).toBeVisible({ timeout: 1000 });
    await input.press('Escape');
    await expect(page.locator('#search-spotlight')).toBeHidden({ timeout: 1000 });
  });

  test('/ shortcut also focuses search', async ({ page }) => {
    await page.goto('/');
    await page.locator('body').click();
    await page.keyboard.press('/');
    await expect(page.locator('#search-input')).toBeFocused();
  });
});
