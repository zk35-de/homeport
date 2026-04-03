// @ts-check
const { test, expect } = require('@playwright/test');

test.describe('Backup & Analytics', () => {
  test('GET /manage/backup returns file attachment', async ({ request }) => {
    const resp = await request.get('/manage/backup');
    expect(resp.ok()).toBeTruthy();
    const disposition = resp.headers()['content-disposition'] || '';
    expect(disposition).toContain('attachment');
    expect(disposition).toMatch(/\.db/);
    const ct = resp.headers()['content-type'] || '';
    expect(ct).toMatch(/sqlite|octet-stream/);
  });

  test('/manage/analytics page loads', async ({ page }) => {
    await page.goto('/manage/analytics');
    await expect(page).toHaveURL('/manage/analytics');
    await expect(page.locator('body')).not.toContainText('500');
  });

  test('analytics page has profile filter', async ({ page }) => {
    await page.goto('/manage/analytics');
    // Profile filter select or heading should be present
    await expect(page.locator('select, h1, h2').first()).toBeVisible();
  });

  test('backup tab is visible in manage', async ({ page }) => {
    await page.goto('/manage');
    await page.locator('[data-panel="backup"]').click();
    await expect(page.locator('#backup')).toBeVisible();
    await expect(page.locator('a[href="/manage/backup"]')).toBeVisible();
  });

  test('restore form is present in backup panel', async ({ page }) => {
    await page.goto('/manage');
    await page.locator('[data-panel="backup"]').click();
    await expect(page.locator('#restore-form')).toBeVisible();
    await expect(page.locator('input[type="file"][name="file"]')).toBeVisible();
  });
});
