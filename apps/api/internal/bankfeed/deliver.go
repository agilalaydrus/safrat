package bankfeed

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type payload struct {
	Source    string     `json:"source"`
	Mutations []Mutation `json:"mutations"`
}

// Result is what the API reported back, so a run can say whether anything
// actually landed rather than only that a request succeeded.
type Result struct {
	Received int `json:"received"`
	Recorded int `json:"recorded"`
	Matched  int `json:"matched"`
}

// Deliver posts credits to the API's bank feed.
//
// Signed over the exact bytes that are sent, computed after marshalling rather
// than from the struct: signing one representation and sending another is how a
// signature ends up proving nothing.
func Deliver(ctx context.Context, endpoint, secret, source string, mutations []Mutation, client *http.Client) (Result, error) {
	if secret == "" {
		return Result{}, fmt.Errorf("BANK_FEED_SECRET kosong; endpoint akan menolak")
	}
	if len(mutations) == 0 {
		return Result{}, nil
	}

	body, err := json.Marshal(payload{Source: source, Mutations: mutations})
	if err != nil {
		return Result{}, fmt.Errorf("encode: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Signature", hex.EncodeToString(mac.Sum(nil)))

	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return Result{}, fmt.Errorf("kirim: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		// The status is the whole diagnosis here: 401 is a wrong secret, 503 is
		// the secret missing on the server, 400 is a malformed batch. Saying
		// which one saves an hour of guessing.
		return Result{}, fmt.Errorf("endpoint menolak dengan status %d", response.StatusCode)
	}

	var result Result
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		// The batch was accepted; only the summary is unreadable. Reported as
		// an error so a run does not claim success it cannot confirm.
		return Result{}, fmt.Errorf("balasan tidak terbaca: %w", err)
	}
	return result, nil
}
