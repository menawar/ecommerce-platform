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
	}
}
