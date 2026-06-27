package main

import "testing"

func TestResolveJWTSecret(t *testing.T) {
	strong := "this-is-a-sufficiently-long-production-secret"

	tests := []struct {
		name        string
		secret      string
		appEnv      string
		wantErr     bool
		wantSecret  string
		wantDevWarn bool
	}{
		{"prod with strong secret", strong, "production", false, strong, false},
		{"prod with empty secret rejected", "", "production", true, "", false},
		{"prod with dev default rejected", devJWTSecret, "production", true, "", false},
		{"prod with too-short secret rejected", "short", "production", true, "", false},
		{"dev empty falls back to dev default", "", "development", false, devJWTSecret, true},
		{"dev with custom secret kept", "my-dev-secret", "development", false, "my-dev-secret", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, devFallback, err := resolveJWTSecret(tc.secret, tc.appEnv)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got secret=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantSecret {
				t.Errorf("secret = %q, want %q", got, tc.wantSecret)
			}
			if devFallback != tc.wantDevWarn {
				t.Errorf("devFallback = %v, want %v", devFallback, tc.wantDevWarn)
			}
		})
	}
}
