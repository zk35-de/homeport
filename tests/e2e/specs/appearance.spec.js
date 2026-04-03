// @ts-check
const { test, expect } = require('@playwright/test');

test.describe('Appearance & Settings', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/manage');
    await page.locator('[data-panel="appearance"]').click();
    await expect(page.locator('#appearance')).toBeVisible();
  });

  test('accent color picker is present', async ({ page }) => {
    await expect(page.locator('#accent-picker')).toBeVisible();
    const value = await page.locator('#accent-picker').inputValue();
    expect(value).toMatch(/^#[0-9a-fA-F]{6}$/);
  });

  test('accent hex label shows current value', async ({ page }) => {
    const pickerVal = await page.locator('#accent-picker').inputValue();
    const label = await page.locator('#accent-hex').textContent();
    expect(label?.trim()).toBe(pickerVal);
  });

  test('theme buttons are present', async ({ page }) => {
    await expect(page.locator('.theme-btn[data-theme="dark"]')).toBeVisible();
    await expect(page.locator('.theme-btn[data-theme="light"]')).toBeVisible();
    await expect(page.locator('.theme-btn[data-theme="system"]')).toBeVisible();
  });

  test('one theme button is active', async ({ page }) => {
    const active = page.locator('.theme-btn.active');
    await expect(active).toHaveCount(1);
  });

  test('custom CSS textarea is present', async ({ page }) => {
    await expect(page.locator('#custom-css-input')).toBeVisible();
    const placeholder = await page.locator('#custom-css-input').getAttribute('placeholder');
    expect(placeholder).toBeTruthy();
  });

  test('save and cancel CSS buttons are present', async ({ page }) => {
    await expect(page.locator('#save-css-btn')).toBeVisible();
    await expect(page.locator('#reset-css-btn')).toBeVisible();
  });

  test('custom CSS is applied to the index page', async ({ page }) => {
    // Write a detectable CSS rule, save it, verify on index page
    const testCSS = '/* e2e-test */ body { --e2e-marker: 1; }';
    await page.locator('#custom-css-input').fill(testCSS);
    await page.locator('#save-css-btn').click();
    await page.waitForTimeout(500);

    await page.goto('/');
    // The custom CSS is injected via <style> or <link>; check it lands in DOM
    const cssContent = await page.evaluate(() => {
      const sheets = Array.from(document.styleSheets);
      for (const sheet of sheets) {
        try {
          const rules = Array.from(sheet.cssRules || []);
          if (rules.some(r => r.cssText?.includes('e2e-marker'))) return true;
        } catch {}
      }
      // Also check <style> tags
      return Array.from(document.querySelectorAll('style')).some(
        s => s.textContent?.includes('e2e-marker')
      );
    });
    expect(cssContent).toBe(true);

    // Cleanup
    await page.goto('/manage');
    await page.locator('[data-panel="appearance"]').click();
    await page.locator('#custom-css-input').fill('');
    await page.locator('#save-css-btn').click();
  });

  test('language selector is present', async ({ page }) => {
    await expect(page.locator('select[name="language"]')).toBeVisible();
    const options = await page.locator('select[name="language"] option').allTextContents();
    expect(options.length).toBeGreaterThanOrEqual(2);
  });
});
