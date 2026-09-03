package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendOTPRequestBoundary(t *testing.T) {
	var calls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/sms/otp" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("missing bearer authentication")
		}
		if r.Header.Get("Idempotency-Key") != "login-19" {
			t.Fatal("missing idempotency key")
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["to"] != "+15551234567" || len(body) != 1 {
			t.Fatalf("body = %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"data":null,"error":{"code":"RATE_LIMITED","message":"retry later"},"metadata":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"id":"otp-19"},"error":null,"metadata":{}}`))
	}))
	defer upstream.Close()

	client := &SMSClient{BaseURL: upstream.URL, APIKey: "test-key", HTTPClient: upstream.Client(), MaxRetries: 1}
	id, err := client.SendOTP(context.Background(), "+15551234567", "login-19")
	if err != nil {
		t.Fatal(err)
	}
	if id != "otp-19" || calls != 2 {
		t.Fatalf("id = %q, calls = %d", id, calls)
	}
}
