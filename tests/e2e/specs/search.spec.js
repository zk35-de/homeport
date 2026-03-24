// @ts-check
const { test, expect } = require('@playwright/test');

test.describe('Search', () => {
  test('search form opens external search in new tab', async ({ page, context }) => {
    await page.goto('/');
    const searchInput = page.locator('#search-input');
    await searchInput.fill('playwright test');

    // Form has target="_blank" → new tab/popup
    const [popup] = await Promise.all([
      context.waitForEvent('page'),
      searchInput.press('Enter'),
    ]);
    await popup.waitForLoadState('commit');
    // Should have navigated to a search engine
    expect(popup.url()).not.toContain('localhost:8855');
    expect(popup.url().length).toBeGreaterThan(10);
  });

  test('bang shortcut !g goes to google', async ({ page, context }) => {
    await page.goto('/');
    const searchInput = page.locator('#search-input');
    await searchInput.fill('!g playwright');

    const [popup] = await Promise.all([
      context.waitForEvent('page'),
      searchInput.press('Enter'),
    ]);
    await popup.waitForLoadState('commit');
    expect(popup.url()).toContain('google.com');
  });

  test('search engine can be switched to Google', async ({ page }) => {
    await page.goto('/');
    await page.locator('#search-engine-btn').click();
    await page.locator('.se-option:has-text("Google")').click();
    // Button label should update to G
    await expect(page.locator('#search-engine-btn')).toContainText('G');
  });

  test('search engine switch to DuckDuckGo', async ({ page }) => {
    await page.goto('/');
    await page.locator('#search-engine-btn').click();
    await page.locator('.se-option:has-text("DuckDuckGo")').click();
    await expect(page.locator('#search-engine-btn')).toContainText('DDG');
  });

  test('Ctrl+K focuses search input', async ({ page }) => {
    await page.goto('/');
    await page.locator('body').click();
    await page.keyboard.press('Control+k');
    await expect(page.locator('#search-input')).toBeFocused();
  });

  test('bang shortcuts are hinted in placeholder', async ({ page }) => {
    await page.goto('/');
    const placeholder = await page.locator('#search-input').getAttribute('placeholder');
    expect(placeholder).toMatch(/!g|!d|Ctrl\+K/);
  });

  test('service search API returns matching services', async ({ request }) => {
    const resp = await request.get('/api/search?q=nginx');
    const results = await resp.json();
    expect(Array.isArray(results)).toBe(true);
    expect(results.length).toBeGreaterThan(0);
    expect(results[0]).toHaveProperty('name');
    expect(results[0]).toHaveProperty('url');
  });
});
