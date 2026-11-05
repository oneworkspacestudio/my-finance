package finance

import (
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func fictionalPushSubscription(t *testing.T) webpush.Subscription {
	t.Helper()
	_, x, y, err := elliptic.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return webpush.Subscription{Endpoint: "https://fcm.googleapis.com/fcm/send/fictional-synthetic-device", Keys: webpush.Keys{
		Auth:   base64.RawURLEncoding.EncodeToString([]byte("fictional-auth16")),
		P256dh: base64.RawURLEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(), x, y)),
	}}
}
func TestCryptoPushEndpointIsBrowserServiceOnly(t *testing.T) {
	for _, endpoint := range []string{
		"http://fcm.googleapis.com/fcm/send/example", "https://127.0.0.1/wpush/example",
		"https://fcm.googleapis.com.evil.invalid/fcm/send/example", "https://fcm.googleapis.com@evil.invalid/fcm/send/example",
		"https://fcm.googleapis.com:444/fcm/send/example", "https://web.push.apple.com/example#fragment",
		"https://fcm.googleapis.com/another-service", "https://notify.windows.com.evil.invalid/example",
		"https://updates.push.services.mozilla.com/admin/example", "https://fcm.googleapis.com/%66cm/send/example",
	} {
		if cryptoPushEndpoint(endpoint) {
			t.Errorf("accepted untrusted push endpoint %q", endpoint)
		}
	}
	for _, endpoint := range []string{
		"https://fcm.googleapis.com/fcm/send/example", "https://fcm.googleapis.com/wp/example",
		"https://updates.push.services.mozilla.com/wpush/v2/example", "https://web.push.apple.com/example",
		"https://wns2.example.notify.windows.com/?token=fictional",
	} {
		if !cryptoPushEndpoint(endpoint) {
			t.Errorf("rejected supported push endpoint %q", endpoint)
		}
	}
	sub := fictionalPushSubscription(t)
	if !cryptoPushValid(sub) {
		t.Fatal("valid generated P-256 subscription rejected")
	}
	sub.Keys.P256dh = base64.RawURLEncoding.EncodeToString(make([]byte, 65))
	if cryptoPushValid(sub) {
		t.Fatal("off-curve key accepted")
	}
	sub = fictionalPushSubscription(t)
	sub.Keys.Auth = "small"
	if cryptoPushValid(sub) {
		t.Fatal("short auth secret accepted")
	}
}
