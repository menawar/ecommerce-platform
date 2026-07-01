package notify

import (
	"context"
	"fmt"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const smtpTimeout = 10 * time.Second

// SMTPSender delivers notifications as real email over SMTP. It targets a plain
// (no-auth) relay — Mailpit locally, a managed relay (SES/SendGrid/Postmark SMTP)
// in prod by swapping addr. It implements the same Sender interface as LogSender,
// so the handler is unchanged.
type SMTPSender struct {
	Addr         string // host:port (e.g. mailpit:1025)
	From         string // header From — may be a display form ("Name <addr>")
	envelopeFrom string // bare addr-spec for the SMTP MAIL FROM command
	// send is the transport, injectable in tests; defaults to smtpSendWithTimeout.
	send func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

func NewSMTPSender(addr, from string) *SMTPSender {
	// The header From may carry a display name; the SMTP envelope sender must be the
	// bare address, or strict relays reject MAIL FROM with a 501.
	envelope := from
	if parsed, err := mail.ParseAddress(from); err == nil {
		envelope = parsed.Address
	}
	return &SMTPSender{Addr: addr, From: from, envelopeFrom: envelope, send: smtpSendWithTimeout}
}

func (s *SMTPSender) Send(_ context.Context, n Notification) error {
	if n.To == "" {
		return fmt.Errorf("smtp: no recipient for event %s", n.EventID)
	}
	sendFn := s.send
	if sendFn == nil {
		sendFn = smtpSendWithTimeout
	}
	envelope := s.envelopeFrom
	if envelope == "" {
		envelope = s.From
	}
	msg := buildMessage(s.From, n.To, n.Subject, n.Body, n.EventID)
	if err := sendFn(s.Addr, nil, envelope, []string{n.To}, msg); err != nil {
		return fmt.Errorf("smtp send to %s: %w", n.To, err)
	}
	return nil
}

// buildMessage assembles a MIME plain-text email. The body is quoted-printable
// encoded so non-ASCII (the ₦ symbol, accented names) survives a 7-bit relay, and
// a Message-ID is included for deliverability. Header values are stripped of CR/LF
// to prevent header injection.
func buildMessage(from, to, subject, body, eventID string) []byte {
	var b strings.Builder
	b.WriteString("From: " + sanitizeHeader(from) + "\r\n")
	b.WriteString("To: " + sanitizeHeader(to) + "\r\n")
	b.WriteString("Subject: " + sanitizeHeader(subject) + "\r\n")
	b.WriteString("Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("Message-ID: <" + messageID(eventID, from) + ">\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	b.WriteString("\r\n")

	var qp strings.Builder
	w := quotedprintable.NewWriter(&qp)
	_, _ = w.Write([]byte(body))
	_ = w.Close()
	b.WriteString(qp.String())
	return []byte(b.String())
}

func sanitizeHeader(v string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(v)
}

// messageID builds "<id@domain>" from the event id (fresh UUID if absent) and the
// From address's domain.
func messageID(eventID, from string) string {
	id := eventID
	if id == "" {
		id = uuid.NewString()
	}
	domain := "plateau.example"
	if parsed, err := mail.ParseAddress(from); err == nil {
		if at := strings.LastIndex(parsed.Address, "@"); at >= 0 {
			domain = parsed.Address[at+1:]
		}
	}
	return id + "@" + domain
}

// smtpSendWithTimeout is like net/smtp.SendMail but bounds the dial and I/O with a
// deadline, so a hung relay can't block the consumer goroutine indefinitely.
func smtpSendWithTimeout(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	conn, err := net.DialTimeout("tcp", addr, smtpTimeout)
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(smtpTimeout))

	host, _, _ := net.SplitHostPort(addr)
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()

	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return err
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
