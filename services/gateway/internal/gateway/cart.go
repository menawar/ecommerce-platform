package gateway

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	cartv1 "github.com/menawar/ecommerce-platform/proto/cart/v1"
)

type cartItemDTO struct {
	ProductID string `json:"product_id"`
	Quantity  int32  `json:"quantity"`
}

type cartDTO struct {
	Items []cartItemDTO `json:"items"`
}

func toCartDTO(c *cartv1.Cart) cartDTO {
	items := make([]cartItemDTO, 0, len(c.GetItems()))
	for _, it := range c.GetItems() {
		items = append(items, cartItemDTO{ProductID: it.GetProductId(), Quantity: it.GetQuantity()})
	}
	return cartDTO{Items: items}
}

// userID pulls the authenticated caller's id from the request context. requireAuth
// guarantees it's present on these routes; the bool guards against a routing slip.
func (h *Handler) userID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, ok := IdentityFrom(r.Context())
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal error")
		return "", false
	}
	return id.UserID, true
}

func (h *Handler) getCart(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.userID(w, r)
	if !ok {
		return
	}
	resp, err := h.carts.GetCart(r.Context(), &cartv1.GetCartRequest{UserId: uid})
	if err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toCartDTO(resp.GetCart()))
}

func (h *Handler) addCartItem(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.userID(w, r)
	if !ok {
		return
	}
	var body struct {
		ProductID string `json:"product_id"`
		Quantity  int32  `json:"quantity"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.carts.AddItem(r.Context(), &cartv1.AddItemRequest{
		UserId: uid, ProductId: body.ProductID, Quantity: body.Quantity,
	})
	if err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toCartDTO(resp.GetCart()))
}

func (h *Handler) updateCartItem(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.userID(w, r)
	if !ok {
		return
	}
	var body struct {
		Quantity int32 `json:"quantity"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.carts.UpdateItem(r.Context(), &cartv1.UpdateItemRequest{
		UserId: uid, ProductId: chi.URLParam(r, "productID"), Quantity: body.Quantity,
	})
	if err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toCartDTO(resp.GetCart()))
}

func (h *Handler) removeCartItem(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.userID(w, r)
	if !ok {
		return
	}
	resp, err := h.carts.RemoveItem(r.Context(), &cartv1.RemoveItemRequest{
		UserId: uid, ProductId: chi.URLParam(r, "productID"),
	})
	if err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toCartDTO(resp.GetCart()))
}
