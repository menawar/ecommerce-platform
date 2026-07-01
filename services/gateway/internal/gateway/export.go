package gateway

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"

	orderv1 "github.com/menawar/ecommerce-platform/proto/order/v1"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
)

// dataExportDTO is a machine-readable copy of a user's personal data — the NDPR/GDPR
// access & portability right, served as a downloadable JSON file.
type dataExportDTO struct {
	ExportedAt string       `json:"exported_at"`
	Profile    profileDTO   `json:"profile"`
	Addresses  []addressDTO `json:"addresses"`
	Orders     []orderDTO   `json:"orders"`
}

type profileDTO struct {
	UserID        string `json:"user_id"`
	Email         string `json:"email"`
	FullName      string `json:"full_name"`
	Role          string `json:"role"`
	EmailVerified bool   `json:"email_verified"`
}

const exportOrderPageSize = 100

// exportData assembles the caller's data (profile, addresses, orders) from the User
// and Order services. The three reads are independent, so we fan out concurrently.
func (h *Handler) exportData(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.userID(w, r)
	if !ok {
		return
	}

	var (
		user   *userv1.GetUserResponse
		addrs  *userv1.ListAddressesResponse
		orders []*orderv1.Order
	)
	g, gctx := errgroup.WithContext(r.Context())
	g.Go(func() (err error) {
		user, err = h.users.GetUser(gctx, &userv1.GetUserRequest{UserId: uid})
		return
	})
	g.Go(func() (err error) {
		addrs, err = h.users.ListAddresses(gctx, &userv1.ListAddressesRequest{UserId: uid})
		return
	})
	g.Go(func() (err error) {
		orders, err = h.allOrdersFor(gctx, uid)
		return
	})
	if err := g.Wait(); err != nil {
		h.writeGRPCError(w, r, err)
		return
	}

	addresses := make([]addressDTO, 0, len(addrs.GetAddresses()))
	for _, a := range addrs.GetAddresses() {
		addresses = append(addresses, toAddressDTO(a))
	}
	ords := make([]orderDTO, 0, len(orders))
	for _, o := range orders {
		ords = append(ords, toOrderDTO(o))
	}

	export := dataExportDTO{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Profile: profileDTO{
			UserID:        user.GetUserId(),
			Email:         user.GetEmail(),
			FullName:      user.GetFullName(),
			Role:          user.GetRole(),
			EmailVerified: user.GetEmailVerified(),
		},
		Addresses: addresses,
		Orders:    ords,
	}
	w.Header().Set("Content-Disposition", `attachment; filename="plateau-data-export.json"`)
	writeJSON(w, http.StatusOK, export)
}

// exportOrderConcurrency bounds the per-order detail fetches so a large history
// doesn't open an unbounded number of concurrent gRPC calls.
const exportOrderConcurrency = 8

// allOrdersFor returns EVERY order for the user WITH its line items — an export must
// be complete. ListOrders returns summaries (no items), so we page through it to
// collect ids, then fetch each order's full detail via GetOrder (bounded concurrency).
// The ids come only from the caller's own ListOrders, so ownership is guaranteed.
func (h *Handler) allOrdersFor(ctx context.Context, uid string) ([]*orderv1.Order, error) {
	var ids []string
	for page := int32(1); ; page++ {
		resp, err := h.orders.ListOrders(ctx, &orderv1.ListOrdersRequest{
			UserId:   uid,
			Page:     page,
			PageSize: exportOrderPageSize,
		})
		if err != nil {
			return nil, err
		}
		batch := resp.GetOrders()
		for _, o := range batch {
			ids = append(ids, o.GetId())
		}
		if len(batch) < exportOrderPageSize {
			break
		}
	}

	full := make([]*orderv1.Order, len(ids))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(exportOrderConcurrency)
	for i, id := range ids {
		g.Go(func() error {
			resp, err := h.orders.GetOrder(gctx, &orderv1.GetOrderRequest{OrderId: id})
			if err != nil {
				return err
			}
			full[i] = resp.GetOrder()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return full, nil
}
