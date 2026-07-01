// Package notify turns domain events into notifications. The Sender interface
// abstracts the delivery channel — the mock LogSender now, SendGrid/Twilio later —
// the same Provider pattern as payment.
package notify

import (
	"context"
	"log/slog"
)

// Notification is a rendered message ready to deliver: a recipient, subject, and
// body, plus metadata for logging/correlation. Template rendering happens in the
// handler (see Render); the Sender only transports.
type Notification struct {
	EventID  string
	Template string
	To       string // recipient email
	Subject  string
	Body     string
}

// Sender delivers a rendered notification over some channel (email here).
type Sender interface {
	Send(ctx context.Context, n Notification) error
}

// LogSender is the dev/CI transport: it logs the rendered email instead of sending
// it. The real transport (SMTPSender, Phase 13.2) implements the same interface.
type LogSender struct {
	Log *slog.Logger
}

func (s LogSender) Send(ctx context.Context, n Notification) error {
	s.Log.InfoContext(ctx, "notification sent (log)",
		"event_id", n.EventID, "template", n.Template, "to", n.To, "subject", n.Subject)
	return nil
}
