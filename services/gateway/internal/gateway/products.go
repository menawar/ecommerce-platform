package gateway

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	productv1 "github.com/menawar/ecommerce-platform/proto/product/v1"
)

// productDTO is the JSON shape the gateway exposes to clients. We map the proto
// to an explicit DTO (rather than serializing the proto struct directly) so the
// REST contract — snake_case field names the frontend depends on — is stable and
// decoupled from proto wire details.
type productDTO struct {
	ID          string `json:"id"`
	SKU         string `json:"sku"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Currency    string `json:"currency"`
	CategoryID  string `json:"category_id"`
	Available   int32  `json:"available"`
	CreatedAt   int64  `json:"created_at"`
	ImageURL    string `json:"image_url"`
}

func toProductDTO(p *productv1.Product) productDTO {
	return productDTO{
		ID:          p.GetId(),
		SKU:         p.GetSku(),
		Name:        p.GetName(),
		Description: p.GetDescription(),
		PriceCents:  p.GetPriceCents(),
		Currency:    p.GetCurrency(),
		CategoryID:  p.GetCategoryId(),
		Available:   p.GetAvailable(),
		CreatedAt:   p.GetCreatedAt(),
		ImageURL:    p.GetImageUrl(),
	}
}

// listProducts: GET /products?page=&page_size=&category_id=&q=
// The gateway forwards query params straight through; the Product service owns
// the defaults/caps, so a missing or junk number becomes 0 and the service
// applies its default.
func (h *Handler) listProducts(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	resp, err := h.products.ListProducts(r.Context(), &productv1.ListProductsRequest{
		Page:       int32(atoiOrZero(qs.Get("page"))),
		PageSize:   int32(atoiOrZero(qs.Get("page_size"))),
		CategoryId: qs.Get("category_id"),
		Search:     qs.Get("q"),
		Sort:       qs.Get("sort"),
	})
	if err != nil {
		h.writeGRPCError(w, r, err)
		return
	}

	products := make([]productDTO, 0, len(resp.GetProducts()))
	for _, p := range resp.GetProducts() {
		products = append(products, toProductDTO(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"products": products,
		"total":    resp.GetTotal(),
	})
}

// getProduct: GET /products/{id}. A NotFound from the service maps to HTTP 404, a
// bad id to 400 — handled by writeGRPCError's status table.
func (h *Handler) getProduct(w http.ResponseWriter, r *http.Request) {
	resp, err := h.products.GetProduct(r.Context(), &productv1.GetProductRequest{
		ProductId: chi.URLParam(r, "id"),
	})
	if err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toProductDTO(resp.GetProduct()))
}

// createProductRequest is the admin JSON body for POST /products. The field
// names are the snake_case REST contract; the gateway forwards them verbatim to
// the Product service, which owns all validation — required sku/name, non-negative
// money, unique sku — so a bad request flows back as 400/409 via writeGRPCError.
type createProductRequest struct {
	SKU             string `json:"sku"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	PriceCents      int64  `json:"price_cents"`
	Currency        string `json:"currency"`         // defaults to NGN in the service when empty
	CategoryID      string `json:"category_id"`      // optional; empty = uncategorized
	InitialQuantity int32  `json:"initial_quantity"` // seeds the inventory row
	ImageURL        string `json:"image_url"`        // optional catalog image URL
}

// createProduct: POST /products (admin only). It decodes the body, calls the
// Product service's CreateProduct RPC, and returns the created product as 201.
// Kept deliberately thin — no business rules here, just protocol translation.
func (h *Handler) createProduct(w http.ResponseWriter, r *http.Request) {
	var req createProductRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.products.CreateProduct(r.Context(), &productv1.CreateProductRequest{
		Sku:             req.SKU,
		Name:            req.Name,
		Description:     req.Description,
		PriceCents:      req.PriceCents,
		Currency:        req.Currency,
		CategoryId:      req.CategoryID,
		InitialQuantity: req.InitialQuantity,
		ImageUrl:        req.ImageURL,
	})
	if err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toProductDTO(resp.GetProduct()))
}

func atoiOrZero(s string) int {
	n, _ := strconv.Atoi(s) // err -> 0, which the service treats as "use default"
	return n
}
