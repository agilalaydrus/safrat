package mailer

import (
	"context"
	"strings"
	"testing"
)

func TestFromEnvRejectsInvalidConfiguration(t *testing.T) {
	if value, err := FromEnv("smtp.example.test", "465", "", "", ""); err != nil || value != nil {
		t.Fatalf("unset SMTP should be disabled without error: value=%#v err=%v", value, err)
	}
	if _, err := FromEnv("smtp.example.test", "25", "sender@example.test", "secret", "sender@example.test"); err == nil {
		t.Fatal("unsafe SMTP port accepted")
	}
	if _, err := FromEnv("smtp.example.test", "465", "sender@example.test", "secret", "sender@example.test\r\nBcc: attacker@example.test"); err == nil {
		t.Fatal("header injection in from address accepted")
	}
}

func TestSendHTMLRejectsHeadersBeforeDial(t *testing.T) {
	client := &SMTP{Host: "does-not-exist.invalid", Port: 465, User: "sender@example.test", Password: "secret", From: "sender@example.test"}
	tests := []struct{ to, subject, messageID string }{
		{"victim@example.test\r\nBcc: attacker@example.test", "Kwitansi", "receipt@example.test"},
		{"victim@example.test", "Kwitansi\r\nBcc: attacker@example.test", "receipt@example.test"},
		{"victim@example.test", "Kwitansi", "receipt@example.test>\r\nBcc: attacker@example.test"},
	}
	for _, test := range tests {
		err := client.SendHTML(context.Background(), test.to, test.subject, "<p>aman</p>", test.messageID)
		if err == nil || !strings.Contains(err.Error(), "email") {
			t.Fatalf("unsafe header not rejected locally: %v", err)
		}
	}
}
