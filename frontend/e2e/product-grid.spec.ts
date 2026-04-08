import { test, expect } from '@playwright/test';
import { MOCK_PRODUCTS, fulfillRows, mockSearchProducts } from './helpers';

test.describe('ProductGrid – AG Grid SSRM', () => {
  test('renders the page header', async ({ page }) => {
    await mockSearchProducts(page, async (_, route) => {
      await fulfillRows(route, MOCK_PRODUCTS);
    });
    await page.goto('/');
    await expect(page.getByText('AG-Grid SSRM — Products POC')).toBeVisible();
  });

  test('loads and displays initial rows from the API', async ({ page }) => {
    await mockSearchProducts(page, async (_, route) => {
      await fulfillRows(route, MOCK_PRODUCTS);
    });
    await page.goto('/');

    // Wait for AG-Grid to finish loading (loading overlay disappears)
    await expect(page.locator('.ag-overlay-loading-wrapper')).toHaveCount(0, { timeout: 10_000 });

    // Each product name should appear as a cell in the grid
    for (const product of MOCK_PRODUCTS) {
      await expect(
        page.locator('[role="gridcell"][col-id="name"]', { hasText: product.name }),
      ).toBeVisible();
    }
  });

  test('displays correct column headers', async ({ page }) => {
    await mockSearchProducts(page, async (_, route) => {
      await fulfillRows(route, MOCK_PRODUCTS);
    });
    await page.goto('/');

    const expectedHeaders = ['Name', 'Category', 'Subcategory', 'Price', 'Quantity', 'Rating'];
    for (const header of expectedHeaders) {
      await expect(
        page.locator('.ag-header-cell-text', { hasText: header }).first(),
      ).toBeVisible();
    }
  });

  test('sends text filter to the API and shows filtered results', async ({ page }) => {
    let capturedRequest: Record<string, unknown> | null = null;

    await mockSearchProducts(page, async (body, route) => {
      capturedRequest = body;
      // Return only matching product when filter is active
      const filterModel = body.filterModel as Record<string, unknown> | undefined;
      if (filterModel && Object.keys(filterModel).length > 0) {
        await fulfillRows(route, [MOCK_PRODUCTS[0]]); // Widget Alpha
      } else {
        await fulfillRows(route, MOCK_PRODUCTS);
      }
    });

    await page.goto('/');
    await expect(page.locator('.ag-overlay-loading-wrapper')).toHaveCount(0, { timeout: 10_000 });

    // Type into the floating filter input for the "name" column using its aria-label
    const nameFilterInput = page.locator('input[aria-label="Name Filter Input"]');
    await nameFilterInput.fill('Widget');
    await nameFilterInput.press('Enter');

    // Wait for re-render after filtering
    await expect(
      page.locator('[role="gridcell"][col-id="name"]', { hasText: 'Widget Alpha' }),
    ).toBeVisible({ timeout: 10_000 });

    // Non-matching products should not be visible
    await expect(
      page.locator('[role="gridcell"][col-id="name"]', { hasText: 'Comfort Chair' }),
    ).not.toBeVisible();

    // Verify the API was called with the filter model
    expect(capturedRequest).not.toBeNull();
    const fm = (capturedRequest as Record<string, unknown>).filterModel as Record<string, unknown>;
    expect(fm).toHaveProperty('name');
  });

  test('sends number filter to the API and shows filtered results', async ({ page }) => {
    let capturedRequest: Record<string, unknown> | null = null;

    await mockSearchProducts(page, async (body, route) => {
      capturedRequest = body;
      const filterModel = body.filterModel as Record<string, unknown> | undefined;
      if (filterModel && Object.keys(filterModel).length > 0) {
        // Return only expensive products
        await fulfillRows(route, MOCK_PRODUCTS.filter((p) => p.price >= 200));
      } else {
        await fulfillRows(route, MOCK_PRODUCTS);
      }
    });

    await page.goto('/');
    await expect(page.locator('.ag-overlay-loading-wrapper')).toHaveCount(0, { timeout: 10_000 });

    // Use the number input for the price floating filter (not the disabled range input)
    const priceFilterInput = page.locator(
      'input[aria-label="Price Filter Input"][type="number"]',
    );
    await priceFilterInput.fill('200');
    await priceFilterInput.press('Enter');

    await expect(
      page.locator('[role="gridcell"][col-id="name"]', { hasText: 'Widget Alpha' }),
    ).toBeVisible({ timeout: 10_000 });
    await expect(
      page.locator('[role="gridcell"][col-id="name"]', { hasText: 'Coffee Maker' }),
    ).not.toBeVisible();

    // Verify the API received a price filter
    expect(capturedRequest).not.toBeNull();
    const fm = (capturedRequest as Record<string, unknown>).filterModel as Record<string, unknown>;
    expect(fm).toHaveProperty('price');
  });

  test('sends sort model to the API when a column header is clicked', async ({ page }) => {
    const capturedRequests: Record<string, unknown>[] = [];

    await mockSearchProducts(page, async (body, route) => {
      capturedRequests.push(body);
      const sortModel = body.sortModel as Array<{ colId: string; sort: string }> | undefined;
      if (sortModel && sortModel.length > 0 && sortModel[0].sort === 'asc') {
        await fulfillRows(route, [...MOCK_PRODUCTS].sort((a, b) => a.name.localeCompare(b.name)));
      } else {
        await fulfillRows(route, MOCK_PRODUCTS);
      }
    });

    await page.goto('/');
    await expect(page.locator('.ag-overlay-loading-wrapper')).toHaveCount(0, { timeout: 10_000 });

    // Click on the "Name" column header (use role="columnheader" to avoid matching
    // the floating filter row which also has col-id="name")
    await page
      .locator('[role="columnheader"][col-id="name"] .ag-header-cell-label')
      .click();

    // Wait for the grid to re-fetch after sorting
    await expect(async () => {
      const sortedRequest = capturedRequests.find((r) => {
        const sm = r.sortModel as Array<{ colId: string; sort: string }> | undefined;
        return sm && sm.length > 0 && sm[0].colId === 'name';
      });
      expect(sortedRequest).toBeDefined();
    }).toPass({ timeout: 10_000 });
  });

  test('paginates to the next page', async ({ page }) => {
    const PAGE_SIZE = 100;
    // Create enough mock rows to have multiple pages
    const manyRows = Array.from({ length: 150 }, (_, i) => ({
      ...MOCK_PRODUCTS[i % MOCK_PRODUCTS.length],
      id: i + 1,
      name: `Product ${String(i + 1).padStart(3, '0')}`,
    }));

    await mockSearchProducts(page, async (body, route) => {
      const startRow = (body.startRow as number) ?? 0;
      const endRow = (body.endRow as number) ?? PAGE_SIZE;
      const slice = manyRows.slice(startRow, endRow);
      const lastRow = startRow + slice.length >= manyRows.length ? manyRows.length : -1;
      await fulfillRows(route, slice, lastRow);
    });

    await page.goto('/');
    await expect(page.locator('.ag-overlay-loading-wrapper')).toHaveCount(0, { timeout: 10_000 });

    // Verify first-page rows are visible
    await expect(
      page.locator('[role="gridcell"][col-id="name"]', { hasText: 'Product 001' }),
    ).toBeVisible();

    // Navigate to next page using the aria-label on the paging button
    await page.locator('[aria-label="Next Page"]').click();

    // After navigation, page 2 rows should be visible
    await expect(
      page.locator('[role="gridcell"][col-id="name"]', { hasText: 'Product 101' }),
    ).toBeVisible({ timeout: 10_000 });
  });

  test('groups rows by category when row grouping is enabled', async ({ page }) => {
    await mockSearchProducts(page, async (body, route) => {
      const rowGroupCols = body.rowGroupCols as Array<{ field: string }> | undefined;
      const groupKeys = body.groupKeys as string[] | undefined;

      if (rowGroupCols && rowGroupCols.length > 0 && groupKeys && groupKeys.length === 0) {
        // First level: return distinct categories
        const categories = [...new Set(MOCK_PRODUCTS.map((p) => p.category))];
        await fulfillRows(
          route,
          categories.map((c) => ({ category: c })),
        );
      } else if (groupKeys && groupKeys.length > 0) {
        // Drill-down: return rows for the selected category
        await fulfillRows(
          route,
          MOCK_PRODUCTS.filter((p) => p.category === groupKeys[0]),
        );
      } else {
        await fulfillRows(route, MOCK_PRODUCTS);
      }
    });

    await page.goto('/');
    await expect(page.locator('.ag-overlay-loading-wrapper')).toHaveCount(0, { timeout: 10_000 });

    // Open the column menu for "Category" using its column header role to avoid
    // accidentally clicking the floating filter cell
    await page.locator('[role="columnheader"][col-id="category"]').hover();
    await page
      .locator('[role="columnheader"][col-id="category"] .ag-header-cell-menu-button')
      .click();

    // Click "Group by Category" in the menu
    await page.getByRole('menuitem', { name: /group by category/i }).click();

    // Category group rows should now appear
    await expect(
      page.locator('[role="row"].ag-row-group', { hasText: 'Electronics' }),
    ).toBeVisible({ timeout: 10_000 });
    await expect(
      page.locator('[role="row"].ag-row-group', { hasText: 'Furniture' }),
    ).toBeVisible();
  });
});
