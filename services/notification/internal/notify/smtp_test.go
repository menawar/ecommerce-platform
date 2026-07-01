package notify

import (
	"context"
	"errors"
	"net/smtp"
	"strings"
	"testing"
)

func TestBuildMessage(t *testing.T) {
	msg := string(buildMessage("Plateau <no-reply@x.com>", "ada@y.com", "Hello", "Line one\nLine two", "evt-1"))

	for _, want := range []string{
		"From: Plateau <no-reply@x.com>\r\n",
		"To: ada@y.com\r\n",
		"Subject: Hello\r\n",
		"MIME-Version: 1.0\r\n",
		"Content-Transfer-Encoding: quoted-printable\r\n",
		"Message-ID: <evt-1@x.com>\r\n",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing header %q", want)
		}
	}
	// Headers and body are separated by a blank line; the ASCII body survives QP.
	if !strings.Contains(msg, "\r\n\r\nLine one") || !strings.Contains(msg, "Line two") {
		t.Errorf("body/headers separation wrong:\n%q", msg)
	}
}

// The ₦ symbol must be quoted-printable encoded (not raw 8-bit) so a 7-bit relay
// can't mangle it.
func TestBuildMessage_NonASCIIEncoded(t *testing.T) {
	msg := string(buildMessage("no-reply@x.com", "ada@y.com", "s", "Total ₦2500.00", "e"))
	if strings.Contains(msg, "₦") {
		t.Error("₦ should be QP-encoded, not written raw")
	}
	if !strings.Contains(msg, "=E2=82=A6") { // UTF-8 bytes of ₦ in quoted-printable
		t.Errorf("₦ should appear QP-encoded as =E2=82=A6; got %q", msg)
	}
}

func TestSanitizeHeader(t *testing.T) {
	if got := sanitizeHeader("a@x.com\r\nBcc: victim@y.com"); strings.ContainsAny(got, "\r\n") {
		t.Errorf("CR/LF must be stripped from header values; got %q", got)
	}
}

// NewSMTPSender splits the display From from the bare envelope address.
func TestNewSMTPSender_EnvelopeFrom(t *testing.T) {
	s := NewSMTPSender("mailpit:1025", "Plateau <no-reply@x.com>")
	if s.From != "Plateau <no-reply@x.com>" {
		t.Errorf("header From = %q", s.From)
	}
	if s.envelopeFrom != "no-reply@x.com" {
		t.Errorf("envelope from = %q, want bare address", s.envelopeFrom)
	}
}

func TestSMTPSender_Send(t *testing.T) {
	var gotAddr, gotFrom string
	var gotTo []string
	var gotMsg []byte
	s := &SMTPSender{
		Addr: "mailpit:1025",
		From: "no-reply@x.com",
		send: func(addr string, _ smtp.Auth, from string, to []string, msg []byte) error {
			gotAddr, gotFrom, gotTo, gotMsg = addr, from, to, msg
			return nil
		},
	}

	err := s.Send(context.Background(), Notification{To: "ada@y.com", Subject: "Hi", Body: "Body here"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotAddr != "mailpit:1025" || gotFrom != "no-reply@x.com" || len(gotTo) != 1 || gotTo[0] != "ada@y.com" {
		t.Errorf("dialed wrong: addr=%s from=%s to=%v", gotAddr, gotFrom, gotTo)
	}
	if !strings.Contains(string(gotMsg), "Subject: Hi") || !strings.Contains(string(gotMsg), "Body here") {
		t.Errorf("message body wrong: %q", gotMsg)
	}
}

func TestSMTPSender_NoRecipient(t *testing.T) {
	s := &SMTPSender{Addr: "x:1025", From: "f@x.com", send: func(string, smtp.Auth, string, []string, []byte) error {
		t.Fatal("send should not be called without a recipient")
		return nil
	}}
	if err := s.Send(context.Background(), Notification{Subject: "s", Body: "b"}); err == nil {
		t.Error("want error when To is empty")
	}
}

func TestSMTPSender_SendError(t *testing.T) {
	s := &SMTPSender{Addr: "x:1025", From: "f@x.com", send: func(string, smtp.Auth, string, []string, []byte) error {
		return errors.New("relay down")
	}}
	if err := s.Send(context.Background(), Notification{To: "a@x.com", Subject: "s", Body: "b"}); err == nil {
		t.Error("want error when the relay fails")
	}
}
