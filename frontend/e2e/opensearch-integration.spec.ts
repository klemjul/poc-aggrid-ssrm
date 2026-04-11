import { test, expect, type Page } from '@playwright/test';

/**
 * Integration tests that run against the real OpenSearch backend — no API
 * mocking. These tests require the backend-opensearch server to be running on
 * http://localhost:8080 with at least some seeded data.
 */

interface SearchRequestBody {
  startRow?: number;
  endRow?: number;
  sortModel?: Array<{ colId: string; sort: string }>;
  filterModel?: Record<string, { filterType?: string; type?: string; filter?: unknown }>;
  rowGroupCols?: Array<{ field?: string }>;
  groupKeys?: string[];
}

interface SearchResponseBody {
  rows: Record<string, unknown>[];
  lastRow: number;
}

/**
 * Waits for a POST /api/search-products round-trip matching the predicate,
 * then returns both the parsed request body and response body.
 */
async function waitForSearchResponse(
  page: Page,
  predicate: (body: SearchRequestBody) => boolean,
): Promise<{ request: SearchRequestBody; response: SearchResponseBody }> {
  const res = await page.waitForResponse((r) => {
    const req = r.request();
    if (!req.url().includes('/api/search-products') || req.method() !== 'POST') return false;
    try {
      return predicate(req.postDataJSON() as SearchRequestBody);
    } catch {
      return false;
    }
  });
  return {
    request: res.request().postDataJSON() as SearchRequestBody,
    response: (await res.json()) as SearchResponseBody,
  };
}

test.describe('ProductGrid – OpenSearch Integration', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('.ag-overlay-loading-wrapper')).toHaveCount(0, {
      timeout: 30_000,
    });
  });

  test('renders the page header', async ({ page }) => {
    await expect(page.getByText('AG-Grid SSRM — Products POC')).toBeVisible();
  });

  test('loads and displays rows from OpenSearch', async ({ page }) => {
    await expect(page.locator('[role="gridcell"][col-id="name"]').first()).toBeVisible({
      timeout: 30_000,
    });
  });

  test('displays correct column headers', async ({ page }) => {
    const expectedHeaders = ['Name', 'Category', 'Subcategory', 'Price', 'Quantity', 'Rating'];
    for (const header of expectedHeaders) {
      await expect(page.locator('.ag-header-cell-text', { hasText: header }).first()).toBeVisible();
    }
  });

  test('text filter sends correct request and returns matching results', async ({ page }) => {
    const responsePromise = waitForSearchResponse(
      page,
      (body) => body.filterModel?.name?.filterType === 'text',
    );

    const nameFilterInput = page.locator('input[aria-label="Name Filter Input"]');
    await nameFilterInput.fill('Premium');
    await nameFilterInput.press('Enter');

    const { request, response } = await responsePromise;

    expect(request.filterModel?.name).toMatchObject({
      filterType: 'text',
      filter: 'Premium',
    });

    // Backend responded with rows whose names contain "Premium"
    expect(response.rows.length).toBeGreaterThan(0);
    for (const row of response.rows) {
      expect(String(row.name).toLowerCase()).toContain('premium');
    }

    await expect(page.locator('[role="gridcell"][col-id="name"]').first()).toBeVisible({
      timeout: 30_000,
    });
  });

  test('number filter sends correct request and restricts results by price', async ({ page }) => {
    const responsePromise = waitForSearchResponse(
      page,
      (body) => body.filterModel?.price?.filterType === 'number',
    );

    const priceFilterInput = page.locator('input[aria-label="Price Filter Input"][type="number"]');
    await priceFilterInput.fill('50');
    await priceFilterInput.press('Enter');

    const { request, response } = await responsePromise;

    expect(request.filterModel?.price).toMatchObject({ filterType: 'number' });
    expect(Number(request.filterModel?.price?.filter)).toBe(50);

    // Every returned row should satisfy the price filter
    for (const row of response.rows) {
      expect(Number(row.price)).toBeLessThanOrEqual(50);
    }
  });

  test('groups rows by category using real OpenSearch data', async ({ page }) => {
    const responsePromise = waitForSearchResponse(
      page,
      (body) => body.rowGroupCols?.[0]?.field === 'category' && (body.groupKeys?.length ?? 0) === 0,
    );

    await page.locator('[role="columnheader"][col-id="category"]').hover();
    await page
      .locator('[role="columnheader"][col-id="category"] .ag-header-cell-menu-button')
      .click();
    await page.getByRole('menuitem', { name: /group by category/i }).click();

    const { response } = await responsePromise;

    // Backend returned group-level rows with category values
    expect(response.rows.length).toBeGreaterThan(0);
    for (const row of response.rows) {
      expect(row.category).toBeDefined();
    }

    await expect(page.locator('[role="gridcell"] .ag-group-value').first()).toBeVisible({
      timeout: 30_000,
    });
  });

  test('sort by price sends correct request and returns ordered results', async ({ page }) => {
    const responsePromise = waitForSearchResponse(
      page,
      (body) => body.sortModel?.some((s) => s.colId === 'price' && s.sort === 'asc') ?? false,
    );

    await page.locator('[role="columnheader"][col-id="price"] .ag-header-cell-label').click();

    const { request, response } = await responsePromise;

    expect(request.sortModel).toEqual(
      expect.arrayContaining([expect.objectContaining({ colId: 'price', sort: 'asc' })]),
    );

    // Backend returned rows in ascending price order
    const prices = response.rows.map((row) => Number(row.price));
    for (let i = 1; i < prices.length; i++) {
      expect(prices[i]).toBeGreaterThanOrEqual(prices[i - 1]);
    }

    await expect(page.locator('[role="gridcell"][col-id="price"]').first()).toBeVisible({
      timeout: 30_000,
    });
  });
});
