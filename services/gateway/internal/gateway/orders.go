package gateway

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	orderv1 "github.com/menawar/ecommerce-platform/proto/order/v1"
)

type orderItemDTO struct {
	ProductID  string `json:"product_id"`
	Name       string `json:"name"`
	PriceCents int64  `json:"price_cents"`
	Quantity   int32  `json:"quantity"`
}

type orderDTO struct {
	ID                 string              `json:"id"`
	Status             string              `json:"status"`
	TotalCents         int64               `json:"total_cents"` // subtotal + shipping
	ShippingCents      int64               `json:"shipping_cents"`
	ShippingMethodName string              `json:"shipping_method_name"`
	ShippingAddress    *shippingAddressDTO `json:"shipping_address,omitempty"`
	Currency           string              `json:"currency"`
	PaymentID          string              `json:"payment_id"`
	CreatedAt          int64               `json:"created_at"`
	Items              []orderItemDTO      `json:"items,omitempty"`
}

type shippingAddressDTO struct {
	Recipient  string `json:"recipient"`
	Phone      string `json:"phone"`
	Line1      string `json:"line1"`
	Line2      string `json:"line2"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

func toOrderDTO(o *orderv1.Order) orderDTO {
	dto := orderDTO{
		ID:                 o.GetId(),
		Status:             o.GetStatus(),
		TotalCents:         o.GetTotalCents(),
		ShippingCents:      o.GetShippingCents(),
		ShippingMethodName: o.GetShippingMethodName(),
		Currency:           o.GetCurrency(),
		PaymentID:          o.GetPaymentId(),
		CreatedAt:          o.GetCreatedAt(),
	}
	// Attach the address block only when present (orders predating 11.1c have none).
	if a := o.GetShippingAddress(); a != nil && a.GetLine1() != "" {
		dto.ShippingAddress = &shippingAddressDTO{
			Recipient: a.GetRecipient(), Phone: a.GetPhone(), Line1: a.GetLine1(), Line2: a.GetLine2(),
			City: a.GetCity(), State: a.GetState(), PostalCode: a.GetPostalCode(), Country: a.GetCountry(),
		}
	}
	for _, it := range o.GetItems() {
		dto.Items = append(dto.Items, orderItemDTO{
			ProductID: it.GetProductId(), Name: it.GetName(),
			PriceCents: it.GetPriceCents(), Quantity: it.GetQuantity(),
		})
	}
	return dto
}

type placeOrderBody struct {
	AddressID        string `json:"address_id"`
	ShippingMethodID string `json:"shipping_method_id"`
}

// placeOrder: POST /orders. The cart is read server-side from the user's session;
// the body carries the chosen address + shipping method, and the Idempotency-Key
// header (per checkout attempt) lets a retried/double-clicked submit resolve to the
// SAME order instead of placing a second one.
func (h *Handler) placeOrder(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.userID(w, r)
	if !ok {
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "Idempotency-Key header is required")
		return
	}
	var body placeOrderBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.AddressID == "" || body.ShippingMethodID == "" {
		writeError(w, http.StatusBadRequest, "address_id and shipping_method_id are required")
		return
	}
	resp, err := h.orders.PlaceOrder(r.Context(), &orderv1.PlaceOrderRequest{
		UserId: uid, IdempotencyKey: key, AddressId: body.AddressID, ShippingMethodId: body.ShippingMethodID,
	})
	if err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"order_id":          resp.GetOrderId(),
		"status":            resp.GetStatus(),
		"authorization_url": resp.GetAuthorizationUrl(),
	})
}

func (h *Handler) listOrders(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.userID(w, r)
	if !ok {
		return
	}
	qs := r.URL.Query()
	resp, err := h.orders.ListOrders(r.Context(), &orderv1.ListOrdersRequest{
		UserId:   uid,
		Page:     int32(atoiOrZero(qs.Get("page"))),
		PageSize: int32(atoiOrZero(qs.Get("page_size"))),
	})
	if err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	orders := make([]orderDTO, 0, len(resp.GetOrders()))
	for _, o := range resp.GetOrders() {
		orders = append(orders, toOrderDTO(o))
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": orders})
}

func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.userID(w, r)
	if !ok {
		return
	}
	resp, err := h.orders.GetOrder(r.Context(), &orderv1.GetOrderRequest{OrderId: chi.URLParam(r, "id")})
	if err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	// AUTHORIZATION: GetOrder fetches by id alone, so the gateway MUST verify the
	// order belongs to the caller — otherwise anyone could read anyone's order by
	// guessing ids. Return 404 (not 403) so we don't even confirm the id exists.
	if resp.GetOrder().GetUserId() != uid {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	writeJSON(w, http.StatusOK, toOrderDTO(resp.GetOrder()))
}
