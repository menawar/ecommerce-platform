// Package notify turns domain events into notifications. The Sender interface
// abstracts the delivery channel — the mock LogSender now, SendGrid/Twilio later —
// the same Provider pattern as payment.
package notify

import (
	"context"
	"log/slog"
)

// Notification is the thing being sent.
type Notification struct {
	EventID  string
	UserID   string
	Channel  string
	Template string
	Link     string // optional action link (e.g. the email-verification URL)
}

// Sender delivers a notification over some channel.
type Sender interface {
	Send(ctx context.Context, n Notification) error
}

// LogSender is the v1 "send": a structured log line. Swapping in a real email/SMS
// adapter is a new implementation of this interface, no handler changes.
type LogSender struct {
	Log *slog.Logger
}

func (s LogSender) Send(ctx context.Context, n Notification) error {
	attrs := []any{"channel", n.Channel, "template", n.Template, "user_id", n.UserID, "event_id", n.EventID}
	// DEV ONLY: logging the action link (e.g. the verification token URL) is a
	// convenience for local testing — it is a live credential and must not reach
	// real logs. The real Sender (SendGrid/Twilio) arrives in Phase 13 and emails
	// the link instead of logging it.
	if n.Link != "" {
		attrs = append(attrs, "link", n.Link)
	}
	s.Log.InfoContext(ctx, "notification sent", attrs...)
	return nil
}
