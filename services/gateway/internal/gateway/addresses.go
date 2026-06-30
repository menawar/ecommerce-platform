package gateway

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
)

// addressDTO is the JSON shape the BFF exchanges with the browser. user_id is
// intentionally omitted — the caller is always the owner (it comes from the JWT).
type addressDTO struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Recipient  string `json:"recipient"`
	Phone      string `json:"phone"`
	Line1      string `json:"line1"`
	Line2      string `json:"line2"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
	IsDefault  bool   `json:"is_default"`
}

// addressInputBody is the create/update request body (mutable fields only).
type addressInputBody struct {
	Label      string `json:"label"`
	Recipient  string `json:"recipient"`
	Phone      string `json:"phone"`
	Line1      string `json:"line1"`
	Line2      string `json:"line2"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
	IsDefault  bool   `json:"is_default"` // honored on create
}

func toAddressDTO(a *userv1.Address) addressDTO {
	return addressDTO{
		ID: a.GetId(), Label: a.GetLabel(), Recipient: a.GetRecipient(), Phone: a.GetPhone(),
		Line1: a.GetLine1(), Line2: a.GetLine2(), City: a.GetCity(), State: a.GetState(),
		PostalCode: a.GetPostalCode(), Country: a.GetCountry(), IsDefault: a.GetIsDefault(),
	}
}

func (b addressInputBody) toProto() *userv1.AddressInput {
	return &userv1.AddressInput{
		Label: b.Label, Recipient: b.Recipient, Phone: b.Phone, Line1: b.Line1, Line2: b.Line2,
		City: b.City, State: b.State, PostalCode: b.PostalCode, Country: b.Country,
	}
}

func (h *Handler) listAddresses(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.userID(w, r)
	if !ok {
		return
	}
	resp, err := h.users.ListAddresses(r.Context(), &userv1.ListAddressesRequest{UserId: uid})
	if err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	out := make([]addressDTO, 0, len(resp.GetAddresses()))
	for _, a := range resp.GetAddresses() {
		out = append(out, toAddressDTO(a))
	}
	writeJSON(w, http.StatusOK, map[string]any{"addresses": out})
}

func (h *Handler) createAddress(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.userID(w, r)
	if !ok {
		return
	}
	var body addressInputBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.users.CreateAddress(r.Context(), &userv1.CreateAddressRequest{
		UserId: uid, Address: body.toProto(), IsDefault: body.IsDefault,
	})
	if err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAddressDTO(resp.GetAddress()))
}

func (h *Handler) updateAddress(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.userID(w, r)
	if !ok {
		return
	}
	var body addressInputBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if _, err := h.users.UpdateAddress(r.Context(), &userv1.UpdateAddressRequest{
		UserId: uid, AddressId: chi.URLParam(r, "id"), Address: body.toProto(),
	}); err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deleteAddress(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.userID(w, r)
	if !ok {
		return
	}
	if _, err := h.users.DeleteAddress(r.Context(), &userv1.DeleteAddressRequest{
		UserId: uid, AddressId: chi.URLParam(r, "id"),
	}); err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setDefaultAddress(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.userID(w, r)
	if !ok {
		return
	}
	if _, err := h.users.SetDefaultAddress(r.Context(), &userv1.SetDefaultAddressRequest{
		UserId: uid, AddressId: chi.URLParam(r, "id"),
	}); err != nil {
		h.writeGRPCError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
