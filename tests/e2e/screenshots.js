// npx node screenshots.js
const { chromium } = require('@playwright/test');
const path = require('path');

const OUT = path.join(__dirname, '../../docs/screenshots');
const BASE = 'http://localhost:8855';

(async () => {
  const browser = await chromium.launch();

  async function shot(name, fn, viewport) {
    const ctx = await browser.newContext({
      viewport: viewport || { width: 1280, height: 800 },
    });
    const page = await ctx.newPage();
    await fn(page);
    await page.screenshot({ path: path.join(OUT, name), fullPage: false });
    await ctx.close();
    console.log('✓', name);
  }

  // ── Dashboard dark ────────────────────────────────────────────────
  await shot('dashboard-dark.png', async (page) => {
    await page.goto(BASE);
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(800);
    // ensure dark theme
    await page.evaluate(() => {
      document.documentElement.dataset.theme = 'dark';
    });
    await page.waitForTimeout(300);
  });

  // ── Dashboard light ───────────────────────────────────────────────
  await shot('dashboard-light.png', async (page) => {
    await page.goto(BASE);
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(800);
    await page.evaluate(() => {
      document.documentElement.dataset.theme = 'light';
    });
    await page.waitForTimeout(300);
  });

  // ── Search spotlight ──────────────────────────────────────────────
  await shot('search-spotlight.png', async (page) => {
    await page.goto(BASE);
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(800);
    await page.evaluate(() => {
      document.documentElement.dataset.theme = 'dark';
    });
    // focus search bar
    await page.click('#search-input');
    await page.type('#search-input', 'gra', { delay: 80 });
    await page.waitForTimeout(400);
  });

  // ── Manage UI ─────────────────────────────────────────────────────
  await shot('manage.png', async (page) => {
    await page.goto(BASE + '/manage');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(800);
    await page.evaluate(() => {
      document.documentElement.dataset.theme = 'dark';
    });
    await page.waitForTimeout(500);
  });

  // ── Mobile ────────────────────────────────────────────────────────
  await shot('mobile.png', async (page) => {
    await page.goto(BASE);
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(800);
    await page.evaluate(() => {
      document.documentElement.dataset.theme = 'dark';
    });
    await page.waitForTimeout(300);
  }, { width: 390, height: 844 });

  await browser.close();
  console.log('\nAll screenshots saved to docs/screenshots/');
})();
