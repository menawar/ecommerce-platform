package server_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
)

const resetTopic = "user.password_reset_requested"

func TestRequestPasswordReset_EnumerationSafe(t *testing.T) {
	ctx := context.Background()
	client, pub := newVerifyTestClient(t)

	// Unknown email: still succeeds, but NO reset event is emitted (nothing to send).
	if _, err := client.RequestPasswordReset(ctx, &userv1.RequestPasswordResetRequest{Email: "nobody@example.com"}); err != nil {
		t.Fatalf("RequestPasswordReset(unknown): %v", err)
	}
	if pub.published(resetTopic) {
		t.Error("a reset event was published for an unknown email — leaks account existence")
	}

	// Known email: succeeds AND emits the reset event.
	registerForVerify(t, client, "ada@example.com")
	if _, err := client.RequestPasswordReset(ctx, &userv1.RequestPasswordResetRequest{Email: "ada@example.com"}); err != nil {
		t.Fatalf("RequestPasswordReset(known): %v", err)
	}
	if !pub.published(resetTopic) {
		t.Error("expected a reset event for a known email")
	}
}

func TestResetPassword_HappyPath_ChangesPasswordAndRevokesSessions(t *testing.T) {
	ctx := context.Background()
	client, pub := newVerifyTestClient(t)
	registerForVerify(t, client, "ada@example.com")

	// Establish a session whose refresh token should be killed by the reset.
	login, err := client.Login(ctx, &userv1.LoginRequest{Email: "ada@example.com", Password: "supersecret"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	oldRefresh := login.GetRefreshToken()

	if _, err := client.RequestPasswordReset(ctx, &userv1.RequestPasswordResetRequest{Email: "ada@example.com"}); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	token := pub.tokenFor(t, resetTopic)

	if _, err := client.ResetPassword(ctx, &userv1.ResetPasswordRequest{Token: token, NewPassword: "brand-new-pw"}); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}

	t.Run("old password no longer works", func(t *testing.T) {
		if _, err := client.Login(ctx, &userv1.LoginRequest{Email: "ada@example.com", Password: "supersecret"}); status.Code(err) != codes.Unauthenticated {
			t.Errorf("old password: want Unauthenticated, got %v", err)
		}
	})
	t.Run("new password works", func(t *testing.T) {
		if _, err := client.Login(ctx, &userv1.LoginRequest{Email: "ada@example.com", Password: "brand-new-pw"}); err != nil {
			t.Errorf("new password Login: %v", err)
		}
	})
	t.Run("pre-existing refresh token is revoked", func(t *testing.T) {
		if _, err := client.RefreshToken(ctx, &userv1.RefreshTokenRequest{RefreshToken: oldRefresh}); status.Code(err) != codes.Unauthenticated {
			t.Errorf("old refresh after reset: want Unauthenticated, got %v", err)
		}
	})
}

func TestResetPassword_TokenIsSingleUse(t *testing.T) {
	ctx := context.Background()
	client, pub := newVerifyTestClient(t)
	registerForVerify(t, client, "ada@example.com")
	if _, err := client.RequestPasswordReset(ctx, &userv1.RequestPasswordResetRequest{Email: "ada@example.com"}); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	token := pub.tokenFor(t, resetTopic)

	if _, err := client.ResetPassword(ctx, &userv1.ResetPasswordRequest{Token: token, NewPassword: "brand-new-pw"}); err != nil {
		t.Fatalf("first ResetPassword: %v", err)
	}
	// Re-using the spent token must fail.
	if _, err := client.ResetPassword(ctx, &userv1.ResetPasswordRequest{Token: token, NewPassword: "another-pw"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("reused token: want InvalidArgument, got %v", err)
	}
}

func TestResetPassword_BadInput(t *testing.T) {
	ctx := context.Background()
	client, _ := newVerifyTestClient(t)

	cases := []struct {
		name        string
		token, pass string
	}{
		{"empty token", "", "brand-new-pw"},
		{"unknown token", "00000000-0000-0000-0000-000000000000", "brand-new-pw"},
		{"short password", "00000000-0000-0000-0000-000000000000", "short"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := client.ResetPassword(ctx, &userv1.ResetPasswordRequest{Token: c.token, NewPassword: c.pass})
			if status.Code(err) != codes.InvalidArgument {
				t.Errorf("want InvalidArgument, got %v", err)
			}
		})
	}
}
