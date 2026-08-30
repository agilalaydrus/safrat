// Package bankfeed reads incoming bank credits and delivers them to the API.
//
// Deliberately a separate process from the API, not a worker task. A statement
// reader is the most fragile thing in this system — it parses somebody else's
// export format or, worse, their HTML — and it may need to run somewhere else
// entirely: a different network, a machine with a browser, a laptop somebody
// runs by hand once a week. Keeping it outside means its failures cannot take
// the API down with them.
package bankfeed

import "time"

// Mutation is one credit as the source reported it.
type Mutation struct {
	// ExternalID must be stable across re-reads of the same statement. It is
	// the only thing standing between a re-import and a second settlement.
	ExternalID  string    `json:"external_id"`
	AmountIDR   int64     `json:"amount_idr"`
	Description string    `json:"description"`
	OccurredAt  time.Time `json:"-"`
	// OccurredAtRFC3339 is what goes on the wire. Kept separate so a source
	// that has no usable timestamp cannot silently send a zero time.
	OccurredAtRFC3339 string `json:"occurred_at"`
}

// Source produces credits. One implementation reads a downloaded statement;
// a bank API or a scraper slots in behind the same interface without anything
// downstream changing.
//
// Sources return only credits. A debit reaching the feed would be recorded as
// money arriving, which is the one mistake this whole path exists to avoid.
type Source interface {
	// Name appears in logs and in the recorded mutation's source field, so it
	// has to say what actually produced the row — "API" and "SCRAPER" carry
	// different weight when somebody is deciding whether to trust an amount.
	Name() string
	Fetch() ([]Mutation, error)
}
