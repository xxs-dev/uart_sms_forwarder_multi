package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestNotificationMessageStringIncludesSIMIdentity(t *testing.T) {
	message := NotificationMessage{
		Type:        "sms",
		From:        "+8613800013800",
		Content:     "验证码 123456",
		Timestamp:   1_753_068_800,
		ModuleID:    "sim1",
		ModuleName:  "短信模块 1",
		ModuleAlias: "英国卡",
		PhoneNumber: "+447700900123",
	}.String()

	for _, expected := range []string{
		"SIM卡: SIM1（英国卡）",
		"本机号码: +447700900123",
		"来自: +8613800013800",
		"验证码 123456",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("notification %q does not contain %q", message, expected)
		}
	}
}

func TestCustomWebhookSupportsSIMIdentityVariables(t *testing.T) {
	var received map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(body, &received); err != nil {
			t.Errorf("decode request body %q: %v", body, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	msg := NotificationMessage{
		Type:        "call",
		From:        "+8613900013900",
		Timestamp:   1_753_068_800,
		ModuleID:    "sim2",
		ModuleName:  "短信模块 2",
		ModuleAlias: "备用卡",
		PhoneNumber: "+447700900456",
	}
	config := map[string]interface{}{
		"url":         server.URL,
		"method":      http.MethodPost,
		"contentType": "application/json",
		"body":        `{"slot":"{{sim_label}}","alias":"{{sim_alias}}","number":"{{sim_number}}","module":"{{module_id}}"}`,
	}
	if err := NewNotifier(zap.NewNop()).sendCustomWebhook(context.Background(), config, msg); err != nil {
		t.Fatalf("send webhook: %v", err)
	}

	want := map[string]string{
		"slot":   "SIM2（备用卡）",
		"alias":  "备用卡",
		"number": "+447700900456",
		"module": "sim2",
	}
	for key, expected := range want {
		if received[key] != expected {
			t.Fatalf("received[%q] = %q, want %q", key, received[key], expected)
		}
	}
}
