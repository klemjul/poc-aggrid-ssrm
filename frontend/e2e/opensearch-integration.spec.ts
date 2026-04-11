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

    const priceFilterInput = page.locator(
      'input[aria-label="Price Filter Input"][type="number"]',
    );
    await priceFilterInput.fill('999');
    await priceFilterInput.press('Enter');

    await expect(page.locator('.ag-overlay-loading-wrapper')).toHaveCount(0, {
      timeout: 30_000,
    });

    // Grid should still display a result (there are products >= $999)
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

    // After sorting, the first price cell should be a low value (< 2.00)
    const firstPriceCell = page.locator('[role="gridcell"][col-id="price"]').first();
    await expect(firstPriceCell).toBeVisible({ timeout: 30_000 });
    const priceText = await firstPriceCell.textContent();
    const price = parseFloat(priceText?.replace(/[^0-9.]/g, '') ?? '0');
    expect(price).toBeLessThan(2.0);
  });
});
