// These mirror the gateway's REST JSON exactly (snake_case keys). The gateway's
// productDTO is the contract; this is its TypeScript shape. Keeping them in lockstep
// is the price of a decoupled contract — worth it for a stable client surface.

export type Product = {
  id: string;
  sku: string;
  name: string;
  description: string;
  price_cents: number;
  currency: string;
  category_id: string;
  available: number;
  created_at: number;
};

export type ProductList = {
  products: Product[];
  total: number;
};
