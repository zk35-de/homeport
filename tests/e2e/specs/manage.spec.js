// @ts-check
const { test, expect } = require('@playwright/test');

test.describe('Manage Page', () => {
  test('manage page loads with category list', async ({ page }) => {
    await page.goto('/manage');
    await expect(page.locator('.category-title').first()).toBeVisible();
  });

  test('manage page shows add category and service forms', async ({ page }) => {
    await page.goto('/manage');
    await expect(page.getByRole('heading', { name: /Kategorie hinzufügen/i })).toBeVisible();
    await expect(page.getByRole('heading', { name: /Dienst hinzufügen/i })).toBeVisible();
  });

  test('can add and delete a category', async ({ page }) => {
    await page.goto('/manage');
    page.on('dialog', d => d.accept());

    const catName = `E2E-Cat-${Date.now()}`;

    // Category form: hx-post to /manage/category
    const catForm = page.locator('form[hx-post="/manage/category"]');
    await catForm.locator('input[name="name"]').fill(catName);
    await catForm.getByRole('button', { name: /Kategorie hinzufügen/i }).click();

    const catTitle = page.locator('.category-title', { hasText: catName });
    await expect(catTitle.first()).toBeVisible({ timeout: 5000 });

    const catContainer = page.locator('.manage-category', {
      has: page.locator('.category-title', { hasText: catName }),
    }).first();
    await catContainer.locator('button.delete').click();

    await expect(page.locator('.category-title', { hasText: catName })).toHaveCount(0, { timeout: 5000 });
  });

  test('search engine selector is present in services panel', async ({ page }) => {
    await page.goto('/manage');
    await expect(page.locator('select[name="search_engine"]')).toBeVisible();
  });

  test('service list shows existing services with edit buttons', async ({ page }) => {
    await page.goto('/manage');
    await expect(page.locator('[hx-get*="service"]').first()).toBeVisible();
  });

  test('discovery source type selector has NPM and Traefik options', async ({ page }) => {
    await page.goto('/manage');
    await page.locator('[data-panel="discovery-sources"]').click();
    const typeSelect = page.locator('select[name="type"]');
    const options = await typeSelect.locator('option').allTextContents();
    expect(options.some(o => o.includes('Traefik'))).toBe(true);
    expect(options.some(o => o.includes('Nginx') || o.includes('NPM'))).toBe(true);
  });

  test('color selector is present in category form', async ({ page }) => {
    await page.goto('/manage');
    const catForm = page.locator('form[hx-post="/manage/category"]');
    const colorOptions = await catForm.locator('select[name="color"] option').allTextContents();
    expect(colorOptions.length).toBeGreaterThan(3);
  });
});
