package notify

import "fmt"

// TemplateData is everything a template can reference: the resolved recipient plus
// whatever the triggering event carried. Fields not relevant to a given template
// are simply zero.
type TemplateData struct {
	RecipientName  string
	ActionURL      string // verification / password-reset link
	OrderID        string
	TotalCents     int64
	Currency       string
	TrackingNumber string
}

// Render turns a template name + data into a plain-text email (subject, body). It
// is pure (no I/O) so it's trivially testable. An unknown template falls back to a
// generic body rather than failing — a missing template must never drop an email.
func Render(template string, d TemplateData) (subject, body string) {
	name := d.RecipientName
	if name == "" {
		name = "there"
	}
	switch template {
	case "welcome":
		return "Welcome to Plateau",
			fmt.Sprintf("Hi %s,\n\nWelcome to Plateau — fresh from the Jos Plateau. Happy shopping!", name)
	case "email_verification":
		return "Verify your email",
			fmt.Sprintf("Hi %s,\n\nPlease verify your email address to start ordering:\n\n%s\n\nThis link expires in 24 hours.", name, d.ActionURL)
	case "password_reset":
		return "Reset your password",
			fmt.Sprintf("Hi %s,\n\nWe received a request to reset your password. Use the link below (valid for 1 hour):\n\n%s\n\nIf you didn't request this, you can ignore this email.", name, d.ActionURL)
	case "payment_received":
		return "We've received your payment",
			fmt.Sprintf("Hi %s,\n\nWe've received your payment of %s for order %s. We're preparing it now.", name, money(d.TotalCents, d.Currency), d.OrderID)
	case "order_confirmation":
		return "Your order is confirmed",
			fmt.Sprintf("Hi %s,\n\nYour order %s is confirmed — total %s. Thank you!", name, d.OrderID, money(d.TotalCents, d.Currency))
	case "order_cancelled":
		return "Your order was cancelled",
			fmt.Sprintf("Hi %s,\n\nOrder %s was cancelled and you were not charged. The reserved stock has been released.", name, d.OrderID)
	case "order_shipped":
		b := fmt.Sprintf("Hi %s,\n\nGood news — order %s has shipped.", name, d.OrderID)
		if d.TrackingNumber != "" {
			b += fmt.Sprintf("\n\nTracking number: %s", d.TrackingNumber)
		}
		return "Your order has shipped", b
	case "order_delivered":
		return "Your order was delivered",
			fmt.Sprintf("Hi %s,\n\nOrder %s has been delivered. Enjoy your harvest!", name, d.OrderID)
	case "order_refunded":
		return "Your order was refunded",
			fmt.Sprintf("Hi %s,\n\nOrder %s has been refunded — %s has been returned to your payment method.", name, d.OrderID, money(d.TotalCents, d.Currency))
	default:
		return "Notification from Plateau",
			fmt.Sprintf("Hi %s,\n\nThere's an update on your Plateau account.", name)
	}
}

// money formats integer minor units as a currency amount. NGN uses the ₦ symbol;
// any other currency falls back to "amount CODE" (e.g. "2500.00 USD").
func money(cents int64, currency string) string {
	if currency == "" {
		currency = "NGN"
	}
	amount := float64(cents) / 100
	if currency == "NGN" {
		return fmt.Sprintf("₦%.2f", amount)
	}
	return fmt.Sprintf("%.2f %s", amount, currency)
}
