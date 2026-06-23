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
	s.Log.InfoContext(ctx, "notification sent",
		"channel", n.Channel, "template", n.Template, "user_id", n.UserID, "event_id", n.EventID)
	return nil
}
