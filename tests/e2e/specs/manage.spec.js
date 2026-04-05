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

  // Structural smoke: all 8 tab panels must be present in the DOM.
  // A missing struct field in ManageData causes the template to abort mid-render —
  // the panels after the category list silently disappear from the HTML.
  // This test catches that class of bug without needing to click anything.
  test('all manage tab panels are present in DOM', async ({ page }) => {
    await page.goto('/manage');
    const panels = [
      '#panel-services',
      '#discovery-sources',
      '#analytics-link',
      '#backup',
      '#appearance',
      '#auth',
      '#profiles',
      '#pages',
    ];
    for (const id of panels) {
      await expect(page.locator(id), `panel ${id} missing from DOM`).toBeAttached();
    }
  });

  // Tab switching: each button must make its panel visible.
  // If a panel is absent from the DOM, clicking the tab button has no effect.
  test('all tab buttons switch to their panel', async ({ page }) => {
    await page.goto('/manage');
    const tabs = [
      { btn: '[data-panel="discovery-sources"]', panel: '#discovery-sources' },
      { btn: '[data-panel="backup"]',            panel: '#backup' },
      { btn: '[data-panel="appearance"]',         panel: '#appearance' },
      { btn: '[data-panel="auth"]',               panel: '#auth' },
      { btn: '[data-panel="profiles"]',           panel: '#profiles' },
      { btn: '[data-panel="pages"]',              panel: '#pages' },
    ];
    for (const { btn, panel } of tabs) {
      await page.locator(btn).click();
      await expect(page.locator(panel), `${panel} not visible after clicking ${btn}`)
        .toBeVisible({ timeout: 2000 });
    }
  });

  test('can add and delete a category', async ({ page }) => {
    await page.goto('/manage');
    page.on('dialog', d => d.accept());

    const catName = `E2E-Cat-${Date.now()}`;

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

  test('auth panel loads password form', async ({ page }) => {
    await page.goto('/manage');
    await page.locator('[data-panel="auth"]').click();
    await expect(page.locator('#auth')).toBeVisible();
    await expect(page.locator('input[name="password"]')).toBeVisible();
  });

  test('profiles panel shows profile list and add form', async ({ page }) => {
    await page.goto('/manage');
    await page.locator('[data-panel="profiles"]').click();
    await expect(page.locator('#profiles')).toBeVisible();
    await expect(page.locator('input[name="slug"]')).toBeVisible();
  });

  test('color selector is present in category form', async ({ page }) => {
    await page.goto('/manage');
    const catForm = page.locator('form[hx-post="/manage/category"]');
    const colorOptions = await catForm.locator('select[name="color"] option').allTextContents();
    expect(colorOptions.length).toBeGreaterThan(3);
  });
});
