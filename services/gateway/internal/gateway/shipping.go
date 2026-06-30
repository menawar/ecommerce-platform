package gateway

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	orderv1 "github.com/menawar/ecommerce-platform/proto/order/v1"
)

type shippingMethodDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	SortOrder   int32  `json:"sort_order"`
	Active      bool   `json:"active"`
}

type shippingMethodInputBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	SortOrder   int32  `json:"sort_order"`
	Active      bool   `json:"active"`
}

func toShippingMethodDTO(m *orderv1.ShippingMethod) shippingMethodDTO {
	return shippingMethodDTO{
		ID: m.GetId(), Name: m.GetName(), Description: m.GetDescription(),
		PriceCents: m.GetPriceCents(), SortOrder: m.GetSortOrder(), Active: m.GetActive(),
	}
}

func (b shippingMethodInputBody) toProto() *orderv1.ShippingMethodInput {
	return &orderv1.ShippingMethodInput{
		Name: b.Name, Description: b.Description, PriceCents: b.PriceCents,
		SortOrder: b.SortOrder, Active: b.Active,
	}
}

// listShippingMethods returns active (sellable) methods by default — the checkout
// view, correct for ANY caller including an admin placing an order. The admin
// management page opts into the full list (incl. disabled) with ?all=true, which
// is only honored for an admin; a customer passing it still gets active-only.
func (h *Handler) listShippingMethods(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	includeInactive := r.URL.Query().Get("all") == "true" && id.Role == "admin"
	resp, err := h.orders.ListShippingMethods(r.Context(), &orderv1.ListShippingMethodsRequest{
		ActiveOnly: !includeInactive,
	})
	if err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	out := make([]shippingMethodDTO, 0, len(resp.GetMethods()))
	for _, m := range resp.GetMethods() {
		out = append(out, toShippingMethodDTO(m))
	}
	writeJSON(w, http.StatusOK, map[string]any{"shipping_methods": out})
}

func (h *Handler) createShippingMethod(w http.ResponseWriter, r *http.Request) {
	var body shippingMethodInputBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.orders.CreateShippingMethod(r.Context(), &orderv1.CreateShippingMethodRequest{Method: body.toProto()})
	if err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toShippingMethodDTO(resp.GetMethod()))
}

func (h *Handler) updateShippingMethod(w http.ResponseWriter, r *http.Request) {
	var body shippingMethodInputBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.orders.UpdateShippingMethod(r.Context(), &orderv1.UpdateShippingMethodRequest{
		Id: chi.URLParam(r, "id"), Method: body.toProto(),
	})
	if err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toShippingMethodDTO(resp.GetMethod()))
}

func (h *Handler) deleteShippingMethod(w http.ResponseWriter, r *http.Request) {
	if _, err := h.orders.DeleteShippingMethod(r.Context(), &orderv1.DeleteShippingMethodRequest{
		Id: chi.URLParam(r, "id"),
	}); err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
