package notify_test

import (
	"strings"
	"testing"

	"github.com/menawar/ecommerce-platform/services/notification/internal/notify"
)

func TestRender(t *testing.T) {
	link := "https://plateau.example/verify-email?token=abc"
	cases := []struct {
		template     string
		data         notify.TemplateData
		wantSubject  string   // exact
		bodyContains []string // substrings the body must include
	}{
		{"welcome", notify.TemplateData{RecipientName: "Ada"}, "Welcome to Plateau", []string{"Ada", "Welcome"}},
		{"email_verification", notify.TemplateData{RecipientName: "Ada", ActionURL: link}, "Verify your email", []string{link}},
		{"password_reset", notify.TemplateData{ActionURL: link}, "Reset your password", []string{link, "there"}},
		{"order_confirmation", notify.TemplateData{OrderID: "o-1", TotalCents: 250000, Currency: "NGN"}, "Your order is confirmed", []string{"o-1", "₦2500.00"}},
		{"order_shipped", notify.TemplateData{OrderID: "o-1", TrackingNumber: "TRK-9"}, "Your order has shipped", []string{"o-1", "TRK-9"}},
		{"order_refunded", notify.TemplateData{OrderID: "o-1", TotalCents: 250000, Currency: "NGN"}, "Your order was refunded", []string{"₦2500.00"}},
		{"unknown_template", notify.TemplateData{RecipientName: "Ada"}, "Notification from Plateau", []string{"Ada"}},
	}
	for _, c := range cases {
		t.Run(c.template, func(t *testing.T) {
			subject, body := notify.Render(c.template, c.data)
			if subject != c.wantSubject {
				t.Errorf("subject = %q, want %q", subject, c.wantSubject)
			}
			for _, want := range c.bodyContains {
				if !strings.Contains(body, want) {
					t.Errorf("body missing %q; got %q", want, body)
				}
			}
		})
	}
}

// order_shipped omits the tracking line when there's no tracking number.
func TestRender_ShippedWithoutTracking(t *testing.T) {
	_, body := notify.Render("order_shipped", notify.TemplateData{OrderID: "o-1"})
	if strings.Contains(body, "Tracking") {
		t.Errorf("no tracking number → body should omit the tracking line; got %q", body)
	}
}

// NGN amounts show only the ₦ symbol, never a duplicate ISO code.
func TestRender_MoneyNoDuplicateCurrency(t *testing.T) {
	_, body := notify.Render("order_confirmation", notify.TemplateData{OrderID: "o-1", TotalCents: 250000, Currency: "NGN"})
	if !strings.Contains(body, "₦2500.00") || strings.Contains(body, "NGN") {
		t.Errorf("want ₦2500.00 and no 'NGN' text; got %q", body)
	}
}
