// @ts-check
const { test, expect } = require('@playwright/test');

test.describe('Profile Management', () => {
  test('profiles tab shows existing profiles', async ({ page }) => {
    await page.goto('/manage');
    await page.locator('[data-panel="profiles"]').click();
    await expect(page.locator('#profiles')).toBeVisible();
    await expect(page.locator('#profile-list .manage-service-item').first()).toBeVisible();
  });

  test('at least one profile is marked as default', async ({ page }) => {
    await page.goto('/manage');
    await page.locator('[data-panel="profiles"]').click();
    await expect(page.locator('#profile-list .tag', { hasText: 'Default' })).toBeVisible();
  });

  test('profile form has name and slug inputs', async ({ page }) => {
    await page.goto('/manage');
    await page.locator('[data-panel="profiles"]').click();
    const form = page.locator('#profiles form').first();
    await expect(form.locator('input[name="name"]')).toBeVisible();
    await expect(form.locator('input[name="slug"]')).toBeVisible();
  });

  test('can add and delete a non-default profile', async ({ page }) => {
    await page.goto('/manage');
    page.on('dialog', d => d.accept());

    await page.locator('[data-panel="profiles"]').click();

    const slug = `e2e-${Date.now()}`;
    const form = page.locator('#profiles form').first();
    await form.locator('input[name="name"]').fill('E2E Test');
    await form.locator('input[name="slug"]').fill(slug);
    await form.getByRole('button', { name: /profil/i }).click();

    // Profile appears in list
    await expect(page.locator('#profile-list').getByText(slug)).toBeVisible({ timeout: 5000 });

    // Delete it (it's not default, so delete button exists)
    const item = page.locator('#profile-list .manage-service-item', {
      has: page.locator('.text-muted', { hasText: slug }),
    });
    await item.locator('button.delete').click();

    await expect(page.locator('#profile-list').getByText(slug)).toHaveCount(0, { timeout: 5000 });
  });

  test('new profile is accessible via its slug URL', async ({ page }) => {
    // Create profile, verify page loads, delete
    await page.goto('/manage');
    page.on('dialog', d => d.accept());

    await page.locator('[data-panel="profiles"]').click();
    const slug = `e2e-url-${Date.now()}`;
    const form = page.locator('#profiles form').first();
    await form.locator('input[name="name"]').fill('E2E URL Test');
    await form.locator('input[name="slug"]').fill(slug);
    await form.getByRole('button', { name: /profil/i }).click();
    await page.locator('#profile-list').getByText(slug).waitFor({ timeout: 5000 });

    // Profile page is accessible
    const resp = await page.request.get(`/${slug}`);
    expect(resp.ok()).toBeTruthy();

    // Cleanup
    const item = page.locator('#profile-list .manage-service-item', {
      has: page.locator('.text-muted', { hasText: slug }),
    });
    await item.locator('button.delete').click();
    await page.locator('#profile-list').getByText(slug).waitFor({ state: 'hidden', timeout: 5000 });
  });

  test('auth panel is accessible from manage', async ({ page }) => {
    await page.goto('/manage');
    await page.locator('[data-panel="auth"]').click();
    await expect(page.locator('#auth')).toBeVisible();
    // HOMEPORT_AUTH description is shown
    await expect(page.locator('#auth').getByText(/HOMEPORT_AUTH/)).toBeVisible();
  });

  test('/login page renders a form', async ({ page }) => {
    await page.goto('/login');
    // With auth disabled, login redirects to / or shows a form
    const url = page.url();
    if (url.includes('/login')) {
      await expect(page.locator('form')).toBeVisible();
    } else {
      // Redirected away → auth not enabled, that's fine
      expect(url).toContain('localhost:8855');
    }
  });
});
