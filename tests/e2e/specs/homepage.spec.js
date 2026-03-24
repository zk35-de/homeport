// @ts-check
const { test, expect } = require('@playwright/test');

test.describe('Homepage', () => {
  test('renders Markus profile with categories', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle(/Markus/);
    await expect(page.getByText('Haus', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Service', { exact: true }).first()).toBeVisible();
  });

  test('shows Markus and Andrea profile links in nav', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('nav').getByText('Markus')).toBeVisible();
    await expect(page.locator('nav').getByText('Andrea')).toBeVisible();
  });

  test('profile switch to Andrea works', async ({ page }) => {
    await page.goto('/');
    await page.locator('a[href="/andrea"]').click();
    await expect(page).toHaveURL(/andrea/);
    await expect(page).toHaveTitle(/Andrea/);
  });

  test('Andrea profile accessible directly', async ({ page }) => {
    await page.goto('/andrea');
    await expect(page).toHaveTitle(/Andrea/);
    // Andrea has different default search engine (DDG shown as DDG)
    await expect(page.locator('#search-engine-btn')).toBeVisible();
  });

  test('dark mode toggle cycles data-theme attribute', async ({ page }) => {
    await page.goto('/');
    const htmlEl = page.locator('html');
    const initial = await htmlEl.getAttribute('data-theme');

    await page.locator('.nav-theme-toggle').click();
    const after = await htmlEl.getAttribute('data-theme');

    // theme should have changed (dark→light or light→system etc.)
    expect(after).not.toBe(initial);
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
