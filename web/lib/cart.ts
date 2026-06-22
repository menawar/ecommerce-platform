import "server-only";

import { gatewayFetch } from "./gateway";

// Mirrors the gateway's cart DTO: ONLY product_id + quantity. No price, no name —
// the cart is intentions, not money (same trust boundary as the backend).
export type CartItem = { product_id: string; quantity: number };
export type Cart = { items: CartItem[] };

export async function getCart(): Promise<Cart> {
  return gatewayFetch<Cart>("/cart");
}

export async function addToCart(productID: string, quantity: number): Promise<Cart> {
  return gatewayFetch<Cart>("/cart/items", {
    method: "POST",
    body: JSON.stringify({ product_id: productID, quantity }),
  });
}

export async function updateCartItem(productID: string, quantity: number): Promise<Cart> {
  return gatewayFetch<Cart>(`/cart/items/${encodeURIComponent(productID)}`, {
    method: "PUT",
    body: JSON.stringify({ quantity }),
  });
}

export async function removeCartItem(productID: string): Promise<Cart> {
  return gatewayFetch<Cart>(`/cart/items/${encodeURIComponent(productID)}`, {
    method: "DELETE",
  });
}
