package gateway_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/menawar/ecommerce-platform/pkg/auth"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
)

// jwtBackedFake returns a fake UserServiceClient whose Login issues a REAL JWT
// and whose ValidateToken verifies it — using the same auth.JWTManager the real
// service uses. This lets us exercise the full token round-trip through the
// gateway's middleware WITHOUT importing the user service's internal packages
// (forbidden across modules). pkg/auth is shared, so the crypto here is real.
func jwtBackedFake(mgr *auth.JWTManager) *fakeUserClient {
	return &fakeUserClient{
		loginFn: func(*userv1.LoginRequest) (*userv1.LoginResponse, error) {
			tok, exp, err := mgr.Issue("user-42", "customer")
			if err != nil {
				return nil, err
			}
			return &userv1.LoginResponse{AccessToken: tok, ExpiresAt: exp.Unix()}, nil
		},
		validateFn: func(in *userv1.ValidateTokenRequest) (*userv1.ValidateTokenResponse, error) {
			claims, err := mgr.Validate(in.GetToken())
			if err != nil {
				return &userv1.ValidateTokenResponse{Valid: false}, nil
			}
			return &userv1.ValidateTokenResponse{Valid: true, UserId: claims.UserID, Role: claims.Role}, nil
		},
	}
}

func getWithAuth(t *testing.T, url, authHeader string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

// TestProtectedRoute is the Phase 1 acceptance, automated: log in to get a real
// token, then the protected /me accepts it and returns the identity, while a bad
// token and a missing header are both rejected with 401.
func TestProtectedRoute(t *testing.T) {
	mgr := auth.NewJWTManager("itest-secret", 15*time.Minute)
	ts := newTestServer(t, jwtBackedFake(mgr))

	// 1) Login -> real access token.
	login := postJSON(t, ts.URL+"/auth/login", `{"email":"a@b.com","password":"supersecret"}`)
	var lr map[string]any
	_ = json.NewDecoder(login.Body).Decode(&lr)
	_ = login.Body.Close()
	token, _ := lr["access_token"].(string)
	if token == "" {
		t.Fatal("login returned no access_token")
	}

	// 2) Valid token -> 200 + identity.
	t.Run("valid token accepted", func(t *testing.T) {
		resp := getWithAuth(t, ts.URL+"/me", "Bearer "+token)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var id map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&id)
		if id["user_id"] != "user-42" || id["role"] != "customer" {
			t.Errorf("identity = %+v, want {user-42 customer}", id)
		}
	})

	// 3) Garbage token -> 401.
	t.Run("bad token rejected", func(t *testing.T) {
		resp := getWithAuth(t, ts.URL+"/me", "Bearer not.a.real.token")
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	})

	// 4) Missing header -> 401.
	t.Run("missing header rejected", func(t *testing.T) {
		resp := getWithAuth(t, ts.URL+"/me", "")
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	})
}
