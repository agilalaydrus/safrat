package payment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreatePayoutUsesV3IdempotencyAndRefundPurpose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("api-version") != "2025-09-01" || r.Header.Get("idempotency-key") != "refund-payout-id" {
			t.Errorf("method/version/key = %s/%s/%s", r.Method, r.Header.Get("api-version"), r.Header.Get("idempotency-key"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["purpose_code"] != "REFUND" || body["source_of_fund"] != "BUSINESS_REVENUE" {
			t.Errorf("classification = %v/%v", body["purpose_code"], body["source_of_fund"])
		}
		recipient := body["recipient"].(map[string]any)
		account := recipient["account_details"].(map[string]any)
		if account["account_number"] != "1234567890" || account["routing_type_1"] != "SWIFT" || account["routing_value_1"] != "CENAIDJA" {
			t.Errorf("account payload = %#v", account)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"payout_id":"po-1","reference_id":"request-1","status":"ACCEPTED","source_amount":250000}`))
	}))
	defer server.Close()
	client := NewClientWithEndpoints("secret", "", server.URL)
	result, err := client.CreatePayout(context.Background(), CreatePayoutRequest{ReferenceID: "request-1", IdempotencyKey: "refund-payout-id", AmountIDR: 250000, GivenName: "Agil", Surname: "Idrus", Phone: "08123456789", AccountHolder: "Agil Idrus", AccountNumber: "1234567890", RoutingType: "SWIFT", RoutingValue: "CENAIDJA", Description: "Refund request"})
	if err != nil {
		t.Fatalf("create payout: %v", err)
	}
	if result.ID != "po-1" || result.Status != "ACCEPTED" || result.AmountIDR != 250000 {
		t.Fatalf("result = %+v", result)
	}
}
