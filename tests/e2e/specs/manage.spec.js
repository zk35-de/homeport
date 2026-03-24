// @ts-check
const { test, expect } = require('@playwright/test');

test.describe('Manage Page', () => {
  test('manage page loads with category list', async ({ page }) => {
    await page.goto('/manage');
    // Category titles appear in the service list section
    await expect(page.locator('.category-title').first()).toBeVisible();
  });

  test('manage page shows add forms', async ({ page }) => {
    await page.goto('/manage');
    await expect(page.getByRole('heading', { name: /Kategorie hinzufügen/i })).toBeVisible();
    await expect(page.getByRole('heading', { name: /Dienst hinzufügen/i })).toBeVisible();
    await expect(page.getByRole('heading', { name: /Widget hinzufügen/i })).toBeVisible();
  });

  test('can add and delete a category', async ({ page }) => {
    await page.goto('/manage');

    // Accept hx-confirm dialogs automatically
    page.on('dialog', d => d.accept());

    // Unique name to avoid collisions with leftovers from previous runs
    const catName = `E2E-Cat-${Date.now()}`;

    // The add-category form is the one with a layout select (unique to category form)
    const catForm = page.locator('form:has(select[name="layout"])');
    await catForm.locator('input[name="name"]').fill(catName);
    await catForm.getByRole('button', { name: /Kategorie hinzufügen/i }).click();

    // Category should appear in the list
    const catTitle = page.locator('.category-title', { hasText: catName });
    await expect(catTitle.first()).toBeVisible({ timeout: 5000 });

    // Delete it — find the specific container and click its delete button
    const catContainer = page.locator('.manage-category', {
      has: page.locator('.category-title', { hasText: catName }),
    }).first();
    await catContainer.locator('button.delete').click();

    // After deletion none should remain (use count check to avoid strict mode issue)
    await expect(page.locator('.category-title', { hasText: catName })).toHaveCount(0, { timeout: 5000 });
  });

  test('search engine section shows per-profile selectors', async ({ page }) => {
    await page.goto('/manage');
    // Search engine section has radio/button groups per profile
    await expect(page.locator('.form-group').filter({ hasText: /markus/i }).first()).toBeVisible();
  });

  test('service list shows existing services with edit buttons', async ({ page }) => {
    await page.goto('/manage');
    // Edit buttons (HTMX hx-get) exist for services
    await expect(page.locator('[hx-get*="service"]').first()).toBeVisible();
  });

  test('widget type selector has expected options', async ({ page }) => {
    await page.goto('/manage');
    // Widget form uses select[name="widget_type"] (not "type" which is for discovery sources)
    const widgetSelect = page.locator('select[name="widget_type"]');
    const options = await widgetSelect.locator('option').allTextContents();
    expect(options.length).toBeGreaterThan(3);
    // Check for known widget types
    expect(options.some(o => /uhr|clock|countdown/i.test(o))).toBe(true);
    expect(options.some(o => /wetter|weather/i.test(o))).toBe(true);
    expect(options.some(o => /rss/i.test(o))).toBe(true);
  });

  test('discovery source type selector has NPM and Traefik options', async ({ page }) => {
    await page.goto('/manage');
    // Discovery source form uses select[name="type"]
    const typeSelect = page.locator('select[name="type"]');
    const options = await typeSelect.locator('option').allTextContents();
    expect(options.some(o => o.includes('Traefik'))).toBe(true);
    expect(options.some(o => o.includes('Nginx') || o.includes('NPM'))).toBe(true);
  });
});
