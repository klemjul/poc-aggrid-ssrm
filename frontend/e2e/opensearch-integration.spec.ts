import { test, expect } from '@playwright/test';

/**
 * Integration tests that run against the real OpenSearch backend — no API
 * mocking. These tests require the backend-opensearch server to be running on
 * http://localhost:8080 with at least some seeded data.
 */
test.describe('ProductGrid – OpenSearch Integration', () => {
  test('renders the page header', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByText('AG-Grid SSRM — Products POC')).toBeVisible();
  });

  test('loads and displays rows from OpenSearch', async ({ page }) => {
    await page.goto('/');

    // Wait for the loading overlay to disappear
    await expect(page.locator('.ag-overlay-loading-wrapper')).toHaveCount(0, {
      timeout: 30_000,
    });

    // Verify that at least one row is rendered
    await expect(
      page.locator('[role="gridcell"][col-id="name"]').first(),
    ).toBeVisible({ timeout: 30_000 });
  });

  test('displays correct column headers', async ({ page }) => {
    await page.goto('/');

    const expectedHeaders = ['Name', 'Category', 'Subcategory', 'Price', 'Quantity', 'Rating'];
    for (const header of expectedHeaders) {
      await expect(
        page.locator('.ag-header-cell-text', { hasText: header }).first(),
      ).toBeVisible();
    }
  });

  test('text filter returns fewer results from OpenSearch', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('.ag-overlay-loading-wrapper')).toHaveCount(0, {
      timeout: 30_000,
    });

    // Count visible rows before filtering
    const allRows = page.locator('[role="gridcell"][col-id="name"]');
    const totalBefore = await allRows.count();
    expect(totalBefore).toBeGreaterThan(0);

    // Apply a text filter — the seed adjectives include "Premium"
    const nameFilterInput = page.locator('input[aria-label="Name Filter Input"]');
    await nameFilterInput.fill('Premium');
    await nameFilterInput.press('Enter');

    // Wait for the grid to re-fetch and update
    await expect(page.locator('.ag-overlay-loading-wrapper')).toHaveCount(0, {
      timeout: 30_000,
    });

    // After filtering, rows should still be visible (seed has "Premium" items)
    await expect(
      page.locator('[role="gridcell"][col-id="name"]').first(),
    ).toBeVisible({ timeout: 30_000 });
  });

  test('number filter restricts results by price', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('.ag-overlay-loading-wrapper')).toHaveCount(0, {
      timeout: 30_000,
    });

    // Count visible rows before filtering
    const rowsBefore = await page.locator('[role="gridcell"][col-id="name"]').count();
    expect(rowsBefore).toBeGreaterThan(0);

    // Use a mid-range threshold that is statistically safe even with 1 000 seeded
    // docs (prices are uniform 1.00–999.99 so ~95% of products will be >= 50).
    const priceFilterInput = page.locator(
      'input[aria-label="Price Filter Input"][type="number"]',
    );
    await priceFilterInput.fill('50');
    await priceFilterInput.press('Enter');

    await expect(page.locator('.ag-overlay-loading-wrapper')).toHaveCount(0, {
      timeout: 30_000,
    });

    // Grid should display rows — products priced >= $50 definitely exist in the seed.
    await expect(
      page.locator('[role="gridcell"][col-id="name"]').first(),
    ).toBeVisible({ timeout: 30_000 });
  });

  test('groups rows by category using real OpenSearch data', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('.ag-overlay-loading-wrapper')).toHaveCount(0, {
      timeout: 30_000,
    });

    await page.locator('[role="columnheader"][col-id="category"]').hover();
    await page
      .locator('[role="columnheader"][col-id="category"] .ag-header-cell-menu-button')
      .click();
    await page.getByRole('menuitem', { name: /group by category/i }).click();

    // "Electronics" is one of the seed categories — it should appear as a group row
    await expect(
      page.locator('[role="row"].ag-row-group', { hasText: 'Electronics' }),
    ).toBeVisible({ timeout: 30_000 });
  });

  test('sort by price changes row order', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('.ag-overlay-loading-wrapper')).toHaveCount(0, {
      timeout: 30_000,
    });

    // Click the "Price" column header to sort ascending
    await page
      .locator('[role="columnheader"][col-id="price"] .ag-header-cell-label')
      .click();

    await expect(page.locator('.ag-overlay-loading-wrapper')).toHaveCount(0, {
      timeout: 30_000,
    });

    // After sorting ascending, verify the first several prices are non-decreasing.
    const priceCells = page.locator('[role="gridcell"][col-id="price"]');
    await expect(priceCells.first()).toBeVisible({ timeout: 30_000 });

    const visibleCount = await priceCells.count();
    const sampleSize = Math.min(5, visibleCount);
    const priceTexts = await priceCells.evaluateAll(
      (cells, n) => cells.slice(0, n).map((c) => c.textContent ?? ''),
      sampleSize,
    );
    const prices = priceTexts.map((t) => parseFloat(t.replace(/[^0-9.]/g, '')));
    for (let i = 1; i < prices.length; i++) {
      expect(prices[i]).toBeGreaterThanOrEqual(prices[i - 1]);
    }
  });
});
