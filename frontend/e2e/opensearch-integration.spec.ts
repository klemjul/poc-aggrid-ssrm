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
  const res = await page.waitForResponse(
    (r) => {
      const req = r.request();
      if (!req.url().includes('/api/search-products') || req.method() !== 'POST') return false;
      try {
        return predicate(req.postDataJSON() as SearchRequestBody);
      } catch {
        return false;
      }
    },
    { timeout: 15_000 },
  );
  return {
    request: res.request().postDataJSON() as SearchRequestBody,
    response: (await res.json()) as SearchResponseBody,
  };
}

/**
 * Applies an AG Grid filter model through the grid API. This avoids relying on
 * floating-filter input DOM that may be hidden in some UI configurations.
 */
async function applyFilterModel(page: Page, model: Record<string, unknown>): Promise<void> {
  const ok = await page.evaluate((nextModel) => {
    function findGridApi(el: Element): { setFilterModel: (m: unknown) => void } | null {
      const elRecord = el as unknown as Record<string, unknown>;
      const keys = Object.keys(elRecord);
      for (const key of keys) {
        if (
          key.startsWith('__reactFiber') ||
          key.startsWith('__reactProps') ||
          key.startsWith('__reactInternals')
        ) {
          let fiber = elRecord[key] as
            | {
                stateNode?: { api?: { setFilterModel: (m: unknown) => void } };
                memoizedProps?: { api?: { setFilterModel: (m: unknown) => void } };
                return?: unknown;
              }
            | undefined;

          while (fiber) {
            if (fiber.stateNode?.api) return fiber.stateNode.api;
            if (fiber.memoizedProps?.api) return fiber.memoizedProps.api;
            fiber = fiber.return as typeof fiber;
          }
        }
      }
      return null;
    }

    const root = document.querySelector('.ag-root-wrapper');
    if (!root) return false;
    const api = findGridApi(root);
    if (!api) return false;

    api.setFilterModel(nextModel);
    return true;
  }, model);

  if (!ok) throw new Error('Could not access AG Grid API to apply filter model');
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

    await applyFilterModel(page, {
      name: {
        filterType: 'text',
        type: 'contains',
        filter: 'Premium',
      },
    });

    const { request, response } = await responsePromise;

    expect(request.filterModel?.name).toMatchObject({
      filterType: 'text',
      type: 'contains',
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

    await applyFilterModel(page, {
      price: {
        filterType: 'number',
        type: 'lessThanOrEqual',
        filter: 50,
      },
    });

    const { request, response } = await responsePromise;

    expect(request.filterModel?.price).toMatchObject({
      filterType: 'number',
      type: 'lessThanOrEqual',
    });
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
