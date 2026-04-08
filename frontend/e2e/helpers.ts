import type { Page, Route } from '@playwright/test';

export interface ProductRow {
  id: number;
  name: string;
  category: string;
  subcategory: string;
  price: number;
  quantity: number;
  rating: number;
  created_at: string;
}

export interface SearchResponse {
  rows: Record<string, unknown>[];
  lastRow: number;
}

export const MOCK_PRODUCTS: ProductRow[] = [
  {
    id: 1,
    name: 'Widget Alpha',
    category: 'Electronics',
    subcategory: 'Gadgets',
    price: 299.99,
    quantity: 50,
    rating: 4.5,
    created_at: '2024-01-15',
  },
  {
    id: 2,
    name: 'Widget Beta',
    category: 'Electronics',
    subcategory: 'Devices',
    price: 149.99,
    quantity: 30,
    rating: 3.8,
    created_at: '2024-02-20',
  },
  {
    id: 3,
    name: 'Comfort Chair',
    category: 'Furniture',
    subcategory: 'Chairs',
    price: 499.99,
    quantity: 10,
    rating: 4.9,
    created_at: '2024-03-10',
  },
  {
    id: 4,
    name: 'Running Shoes',
    category: 'Sports',
    subcategory: 'Footwear',
    price: 89.99,
    quantity: 100,
    rating: 4.2,
    created_at: '2024-04-05',
  },
  {
    id: 5,
    name: 'Coffee Maker',
    category: 'Kitchen',
    subcategory: 'Appliances',
    price: 79.99,
    quantity: 75,
    rating: 4.0,
    created_at: '2024-05-12',
  },
];

/** Intercepts POST /api/search-products and calls the provided handler. */
export async function mockSearchProducts(
  page: Page,
  handler: (body: Record<string, unknown>, route: Route) => Promise<void>,
) {
  await page.route('**/api/search-products', async (route) => {
    const body = route.request().postDataJSON() as Record<string, unknown>;
    await handler(body, route);
  });
}

/** Returns a simple fulfilled response with the given rows. */
export async function fulfillRows(
  route: Route,
  rows: Record<string, unknown>[],
  lastRow = rows.length,
) {
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ rows, lastRow } satisfies SearchResponse),
  });
}
