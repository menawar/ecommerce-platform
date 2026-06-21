package server_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/menawar/ecommerce-platform/pkg/auth"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/user/internal/server"
	"github.com/menawar/ecommerce-platform/services/user/internal/store"
)

// newTestClient wires a real Server (in-memory repo + real JWT) behind a bufconn
// gRPC connection and returns a client. This is an integration test of the whole
// service stack minus the network and database.
func newTestClient(t *testing.T) userv1.UserServiceClient {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	jwtMgr := auth.NewJWTManager("test-secret", 15*time.Minute)
	refreshMgr := auth.NewJWTManager("test-secret", 7*24*time.Hour)
	srv := server.NewServer(store.NewMemory(), jwtMgr, refreshMgr, jwtMgr, log)

	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer()
	userv1.RegisterUserServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return userv1.NewUserServiceClient(conn)
}

func TestRegister_Login_Validate_HappyPath(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)

	reg, err := client.Register(ctx, &userv1.RegisterRequest{
		Email: "ada@example.com", Password: "supersecret", FullName: "Ada Lovelace",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if reg.GetUserId() == "" {
		t.Fatal("Register returned empty user_id")
	}

	login, err := client.Login(ctx, &userv1.LoginRequest{Email: "ada@example.com", Password: "supersecret"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if login.GetAccessToken() == "" || login.GetRefreshToken() == "" {
		t.Fatal("Login returned empty tokens")
	}

	val, err := client.ValidateToken(ctx, &userv1.ValidateTokenRequest{Token: login.GetAccessToken()})
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if !val.GetValid() {
		t.Fatal("access token reported invalid")
	}
	if val.GetUserId() != reg.GetUserId() || val.GetRole() != "customer" {
		t.Errorf("claims = {%s %s}, want {%s customer}", val.GetUserId(), val.GetRole(), reg.GetUserId())
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	req := &userv1.RegisterRequest{Email: "dup@example.com", Password: "supersecret", FullName: "Dup"}

	if _, err := client.Register(ctx, req); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	_, err := client.Register(ctx, req)
	if status.Code(err) != codes.AlreadyExists {
		t.Errorf("want AlreadyExists, got %v", err)
	}
}

func TestRegister_Validation(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)

	cases := []struct {
		name string
		req  *userv1.RegisterRequest
	}{
		{"bad email", &userv1.RegisterRequest{Email: "not-an-email", Password: "supersecret", FullName: "X"}},
		{"short password", &userv1.RegisterRequest{Email: "a@b.com", Password: "short", FullName: "X"}},
		{"empty full name", &userv1.RegisterRequest{Email: "a@b.com", Password: "supersecret", FullName: "  "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := client.Register(ctx, tc.req); status.Code(err) != codes.InvalidArgument {
				t.Errorf("want InvalidArgument, got %v", err)
			}
		})
	}
}

// TestLogin_BadCredentials proves the enumeration defense: a wrong password and
// an unknown email return the EXACT same code and message.
func TestLogin_BadCredentials(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	_, _ = client.Register(ctx, &userv1.RegisterRequest{Email: "real@example.com", Password: "supersecret", FullName: "Real"})

	wrongPw, errWrong := client.Login(ctx, &userv1.LoginRequest{Email: "real@example.com", Password: "WRONG"})
	unknown, errUnknown := client.Login(ctx, &userv1.LoginRequest{Email: "ghost@example.com", Password: "whatever1"})

	if wrongPw != nil || unknown != nil {
		t.Fatal("expected nil responses on failed login")
	}
	if status.Code(errWrong) != codes.Unauthenticated || status.Code(errUnknown) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated for both, got %v / %v", errWrong, errUnknown)
	}
	if errWrong.Error() != errUnknown.Error() {
		t.Errorf("error messages differ — leaks account existence:\n wrong:   %q\n unknown: %q", errWrong, errUnknown)
	}
}

func TestValidateToken_Garbage(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)

	val, err := client.ValidateToken(ctx, &userv1.ValidateTokenRequest{Token: "garbage.token.here"})
	if err != nil {
		t.Fatalf("ValidateToken returned RPC error (should return valid=false): %v", err)
	}
	if val.GetValid() {
		t.Error("garbage token reported valid")
	}
}
