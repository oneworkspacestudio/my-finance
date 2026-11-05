package finance

import (
	"strings"
	"testing"
)

func TestCryptoEmailAddressesAndKeysRejectHeaderInjection(t *testing.T) {
	for _, value := range []string{"owner@example.com", "alerts+finance@sub.example.com"} {
		if !cryptoEmailAddress(value) {
			t.Fatal("plain address rejected", value)
		}
	}
	for _, value := range []string{"", "Name <owner@example.com>", "one@example.com,two@example.com", "one@example.com;two@example.com", "one@example.com\r\nBcc: other@example.com", "one\t@example.com", " owner@example.com", "one@example.com ", "\"one\"@example.com", "one(comment)@example.com", "é@example.com", strings.Repeat("a", 245) + "@example.com"} {
		if cryptoEmailAddress(value) {
			t.Fatal("unsupported/injected address", value)
		}
	}
	for _, key := range []string{"short", strings.Repeat("x", 513), "key\r\nsecret", "key secret", "key\tsecret", "key\x00secret"} {
		if cryptoEmailKey(key) {
			t.Fatal("invalid key accepted")
		}
	}
	if !cryptoEmailKey("fictional-resend-key") {
		t.Fatal("fixture key rejected")
	}
}
