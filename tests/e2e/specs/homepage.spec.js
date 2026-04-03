// @ts-check
const { test, expect } = require('@playwright/test');

test.describe('Homepage', () => {
  test('default profile loads at /', async ({ page }) => {
    const resp = await page.goto('/');
    expect(resp.status()).toBeLessThan(400);
    await expect(page.locator('#search-input')).toBeVisible();
  });

  test('nav shows at least one profile link', async ({ page }) => {
    await page.goto('/');
    // Nav contains profile links
    await expect(page.locator('nav a[href^="/"]').first()).toBeVisible();
  });

  test('profile links in nav switch profiles', async ({ page }) => {
    await page.goto('/');
    const links = page.locator('nav a[href^="/"]').filter({ hasNot: page.locator('[href="/manage"]') });
    const count = await links.count();
    if (count > 1) {
      const secondHref = await links.nth(1).getAttribute('href');
      await links.nth(1).click();
      await expect(page).toHaveURL(new RegExp(secondHref.replace('/', '\\/') + '$'));
    }
  });

  test('dark mode toggle button is present and clickable', async ({ page }) => {
    await page.goto('/');
    const toggle = page.locator('.nav-theme-toggle');
    await expect(toggle).toBeVisible();
    // Click toggles — no error thrown
    await toggle.click();
    await expect(page.locator('html')).toBeVisible();
  });

  test('search input is present', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('#search-input')).toBeVisible();
  });

  test('Ctrl+K focuses search input', async ({ page }) => {
    await page.goto('/');
    await page.locator('body').click();
    await page.keyboard.press('Control+k');
    await expect(page.locator('#search-input')).toBeFocused();
  });

  test('search engine selector shows dropdown on click', async ({ page }) => {
    await page.goto('/');
    await page.locator('#search-engine-btn').click();
    await expect(page.locator('.se-option').first()).toBeVisible();
  });

  test('manage link navigates to manage page', async ({ page }) => {
    await page.goto('/');
    await page.locator('a[href="/manage"]').click();
    await expect(page).toHaveURL('/manage');
  });
});
