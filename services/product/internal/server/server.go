// Package server implements the product.v1.ProductService gRPC server over the
// sqlc-generated queries. It maps pgx/pgtype values to proto, runs CreateProduct
// in a transaction, and translates DB outcomes into gRPC status codes.
package server

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	productv1 "github.com/menawar/ecommerce-platform/proto/product/v1"
	"github.com/menawar/ecommerce-platform/services/product/internal/db"
	"github.com/menawar/ecommerce-platform/services/product/internal/inventory"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// Server holds the pool (for transactions) and a Queries bound to the pool (for
// single-statement reads). sqlc's DBTX lets the same Queries run on the pool or,
// via WithTx, inside a transaction.
type Server struct {
	productv1.UnimplementedProductServiceServer
	pool     *pgxpool.Pool
	q        *db.Queries
	reserver *inventory.Reserver
	log      *slog.Logger
}

func NewServer(pool *pgxpool.Pool, log *slog.Logger) *Server {
	return &Server{pool: pool, q: db.New(pool), reserver: inventory.NewReserver(pool), log: log}
}

// CreateProduct inserts the product AND its inventory row atomically. Both must
// succeed or neither does — a product with no inventory row would break every
// stock query — so they run in ONE transaction.
func (s *Server) CreateProduct(ctx context.Context, req *productv1.CreateProductRequest) (*productv1.CreateProductResponse, error) {
	if req.GetSku() == "" || req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "sku and name are required")
	}
	if req.GetPriceCents() < 0 || req.GetInitialQuantity() < 0 {
		return nil, status.Error(codes.InvalidArgument, "price_cents and initial_quantity must be non-negative")
	}
	categoryID, err := parseOptionalUUID(req.GetCategoryId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid category_id")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, s.internal(ctx, "begin tx", err)
	}
	// Rollback is a no-op once Commit has succeeded, so this defer safely covers
	// every early-return path without double-handling the happy path.
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.q.WithTx(tx)

	currency := req.GetCurrency()
	if currency == "" {
		currency = "NGN"
	}

	product, err := qtx.CreateProduct(ctx, db.CreateProductParams{
		Sku:         req.GetSku(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
		PriceCents:  req.GetPriceCents(),
		Currency:    currency,
		CategoryID:  categoryID,
		ImageUrl:    req.GetImageUrl(),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, status.Error(codes.AlreadyExists, "a product with this sku already exists")
		}
		return nil, s.internal(ctx, "create product", err)
	}

	inv, err := qtx.CreateInventory(ctx, db.CreateInventoryParams{
		ProductID: product.ID,
		Quantity:  req.GetInitialQuantity(),
	})
	if err != nil {
		return nil, s.internal(ctx, "create inventory", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, s.internal(ctx, "commit tx", err)
	}

	s.log.InfoContext(ctx, "created product", "product_id", uuidString(product.ID), "sku", product.Sku)
	return &productv1.CreateProductResponse{
		Product: productFromModel(product, inv.Quantity-inv.Reserved),
	}, nil
}

// UpdateProduct full-replaces a product's mutable fields and sets an absolute new
// stock level, in ONE transaction so the catalog row and inventory never disagree.
// sku is immutable (not in the request). Setting stock below the reserved units is
// rejected by the DB CHECK and surfaced as FailedPrecondition.
func (s *Server) UpdateProduct(ctx context.Context, req *productv1.UpdateProductRequest) (*productv1.UpdateProductResponse, error) {
	id, err := parseRequiredUUID(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.GetPriceCents() < 0 {
		return nil, status.Error(codes.InvalidArgument, "price_cents must be non-negative")
	}
	// quantity < 0 is the "leave inventory unchanged" sentinel (so editing catalog
	// fields needn't know — or disturb — the current stock level).
	categoryID, err := parseOptionalUUID(req.GetCategoryId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid category_id")
	}

	currency := req.GetCurrency()
	if currency == "" {
		currency = "NGN"
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, s.internal(ctx, "begin tx", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.q.WithTx(tx)

	product, err := qtx.UpdateProduct(ctx, db.UpdateProductParams{
		ID:          id,
		Name:        req.GetName(),
		Description: req.GetDescription(),
		PriceCents:  req.GetPriceCents(),
		Currency:    currency,
		CategoryID:  categoryID,
		ImageUrl:    req.GetImageUrl(),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "product not found")
		}
		return nil, s.internal(ctx, "update product", err)
	}

	// Touch inventory only when a non-negative quantity was supplied; otherwise read
	// the current row so the response still reports the right available count.
	var available int32
	if q := req.GetQuantity(); q >= 0 {
		inv, err := qtx.SetInventoryQuantity(ctx, db.SetInventoryQuantityParams{ProductID: id, Quantity: q})
		if err != nil {
			if isCheckViolation(err) {
				return nil, status.Error(codes.FailedPrecondition, "cannot set stock below the currently reserved units")
			}
			return nil, s.internal(ctx, "set inventory quantity", err)
		}
		available = inv.Quantity - inv.Reserved
	} else {
		inv, err := qtx.GetInventory(ctx, id)
		if err != nil {
			return nil, s.internal(ctx, "get inventory", err)
		}
		available = inv.Quantity - inv.Reserved
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, s.internal(ctx, "commit tx", err)
	}

	s.log.InfoContext(ctx, "updated product", "product_id", uuidString(product.ID))
	return &productv1.UpdateProductResponse{
		Product: productFromModel(product, available),
	}, nil
}

// DeleteProduct soft-deletes (archives) a product: it leaves the catalog but its
// rows stay so order/reservation history keeps referencing them. An already-archived
// or unknown id is a NotFound (ArchiveProduct returns no row in that case).
func (s *Server) DeleteProduct(ctx context.Context, req *productv1.DeleteProductRequest) (*productv1.DeleteProductResponse, error) {
	id, err := parseRequiredUUID(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}
	if _, err := s.q.ArchiveProduct(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "product not found")
		}
		return nil, s.internal(ctx, "archive product", err)
	}
	s.log.InfoContext(ctx, "archived product", "product_id", uuidString(id))
	return &productv1.DeleteProductResponse{}, nil
}

func (s *Server) GetProduct(ctx context.Context, req *productv1.GetProductRequest) (*productv1.GetProductResponse, error) {
	id, err := parseRequiredUUID(req.GetProductId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid product_id")
	}

	row, err := s.q.GetProductWithInventory(ctx, id)
	if err != nil {
		// pgx returns ErrNoRows for a :one query that matched nothing — the
		// canonical "not found", mapped to gRPC NotFound.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "product not found")
		}
		return nil, s.internal(ctx, "get product", err)
	}

	return &productv1.GetProductResponse{Product: rowFromDetail(row)}, nil
}

func (s *Server) ListProducts(ctx context.Context, req *productv1.ListProductsRequest) (*productv1.ListProductsResponse, error) {
	page := req.GetPage()
	if page < 1 {
		page = 1
	}
	size := req.GetPageSize()
	if size < 1 {
		size = defaultPageSize
	}
	if size > maxPageSize {
		size = maxPageSize
	}
	categoryID, err := parseOptionalUUID(req.GetCategoryId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid category_id")
	}
	// Empty search -> nil pointer -> SQL NULL -> the filter short-circuits (no
	// filtering). A non-empty value becomes a bound ILIKE parameter.
	var search *string
	if s := req.GetSearch(); s != "" {
		search = &s
	}

	rows, err := s.q.ListProductsWithInventory(ctx, db.ListProductsWithInventoryParams{
		CategoryID: categoryID,
		Search:     search,
		Sort:       normalizeSort(req.GetSort()),
		Limit:      size,
		Offset:     (page - 1) * size,
	})
	if err != nil {
		return nil, s.internal(ctx, "list products", err)
	}

	total, err := s.q.CountProducts(ctx, db.CountProductsParams{CategoryID: categoryID, Search: search})
	if err != nil {
		return nil, s.internal(ctx, "count products", err)
	}

	products := make([]*productv1.Product, 0, len(rows))
	for _, r := range rows {
		products = append(products, rowFromList(r))
	}
	return &productv1.ListProductsResponse{Products: products, Total: total}, nil
}

// normalizeSort allow-lists the sort keys the query understands. Anything else
// (empty, unknown, a typo) collapses to "" — the query's default newest-first
// ordering — so a bad value never errors, it just falls back.
func normalizeSort(sort string) string {
	switch sort {
	case "price_asc", "price_desc":
		return sort
	default:
		return ""
	}
}

func (s *Server) internal(ctx context.Context, msg string, err error) error {
	s.log.ErrorContext(ctx, msg, "err", err)
	return status.Error(codes.Internal, "internal error")
}

// isUniqueViolation reports whether err is a Postgres unique-constraint violation
// (SQLSTATE 23505). errors.As walks the wrapped chain to pull out the typed pgx
// error so we can inspect its SQLSTATE code.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// isCheckViolation reports whether err is a Postgres CHECK-constraint violation
// (SQLSTATE 23514) — e.g. setting inventory.quantity below reserved.
func isCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}

// --- pgtype <-> proto mappers ---

func parseRequiredUUID(s string) (pgtype.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}

// parseOptionalUUID treats "" as SQL NULL (an invalid/absent UUID).
func parseOptionalUUID(s string) (pgtype.UUID, error) {
	if s == "" {
		return pgtype.UUID{}, nil
	}
	return parseRequiredUUID(s)
}

func uuidString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

func tsUnix(t pgtype.Timestamptz) int64 {
	if !t.Valid {
		return 0
	}
	return t.Time.Unix()
}

func productFromModel(p db.Product, available int32) *productv1.Product {
	return &productv1.Product{
		Id:          uuidString(p.ID),
		Sku:         p.Sku,
		Name:        p.Name,
		Description: p.Description,
		PriceCents:  p.PriceCents,
		Currency:    p.Currency,
		CategoryId:  uuidString(p.CategoryID),
		Available:   available,
		CreatedAt:   tsUnix(p.CreatedAt),
		ImageUrl:    p.ImageUrl,
	}
}

func rowFromDetail(r db.GetProductWithInventoryRow) *productv1.Product {
	return &productv1.Product{
		Id:          uuidString(r.ID),
		Sku:         r.Sku,
		Name:        r.Name,
		Description: r.Description,
		PriceCents:  r.PriceCents,
		Currency:    r.Currency,
		CategoryId:  uuidString(r.CategoryID),
		Available:   r.Available,
		CreatedAt:   tsUnix(r.CreatedAt),
		ImageUrl:    r.ImageUrl,
	}
}

func rowFromList(r db.ListProductsWithInventoryRow) *productv1.Product {
	return &productv1.Product{
		Id:          uuidString(r.ID),
		Sku:         r.Sku,
		Name:        r.Name,
		Description: r.Description,
		PriceCents:  r.PriceCents,
		Currency:    r.Currency,
		CategoryId:  uuidString(r.CategoryID),
		Available:   r.Available,
		CreatedAt:   tsUnix(r.CreatedAt),
		ImageUrl:    r.ImageUrl,
	}
}
