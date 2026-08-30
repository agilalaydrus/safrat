package bankfeed

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// CSVSource reads a statement exported from internet banking.
//
// This exists because it works today, with no API access and no credentials:
// every Indonesian bank can export a statement, and somebody downloading one
// weekly is a real workflow rather than a placeholder. A bank API implements
// the same interface later without anything downstream changing.
type CSVSource struct {
	Reader io.Reader
	// Column names, because no two banks agree on them. Matched
	// case-insensitively against the header row.
	DateColumn        string
	AmountColumn      string
	DescriptionColumn string
	// Optional. When the export carries the bank's own reference number, that
	// is the right external id. Without one, see deriveID.
	ReferenceColumn string
	// Layout for the date column, e.g. "02/01/2006".
	DateLayout string
	// SourceName is "SCRAPER" or "API" — never invented here, because the
	// weight a person gives an amount depends on where it came from.
	SourceName string
}

func (c *CSVSource) Name() string { return c.SourceName }

func (c *CSVSource) Fetch() ([]Mutation, error) {
	reader := csv.NewReader(c.Reader)
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("baca csv: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("csv tidak punya baris data")
	}

	index := map[string]int{}
	for i, header := range rows[0] {
		index[strings.ToLower(strings.TrimSpace(header))] = i
	}
	column := func(name string) (int, error) {
		if name == "" {
			return -1, nil
		}
		i, ok := index[strings.ToLower(strings.TrimSpace(name))]
		if !ok {
			return 0, fmt.Errorf("kolom %q tidak ada di csv", name)
		}
		return i, nil
	}

	dateAt, err := column(c.DateColumn)
	if err != nil {
		return nil, err
	}
	amountAt, err := column(c.AmountColumn)
	if err != nil {
		return nil, err
	}
	descriptionAt, err := column(c.DescriptionColumn)
	if err != nil {
		return nil, err
	}
	referenceAt, err := column(c.ReferenceColumn)
	if err != nil {
		return nil, err
	}

	mutations := make([]Mutation, 0, len(rows)-1)
	for line, row := range rows[1:] {
		cell := func(at int) string {
			if at < 0 || at >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[at])
		}

		amount, err := parseAmount(cell(amountAt))
		if err != nil {
			return nil, fmt.Errorf("baris %d: %w", line+2, err)
		}
		// Credits only. A debit delivered here would be recorded as money
		// arriving, which is the single mistake this path exists to avoid —
		// so a negative amount is refused rather than made positive.
		if amount <= 0 {
			continue
		}

		occurred, err := time.Parse(c.DateLayout, cell(dateAt))
		if err != nil {
			return nil, fmt.Errorf("baris %d: tanggal %q tidak sesuai format %q", line+2, cell(dateAt), c.DateLayout)
		}

		description := cell(descriptionAt)
		id := cell(referenceAt)
		if id == "" {
			id = deriveID(occurred, amount, description)
		}

		mutations = append(mutations, Mutation{
			ExternalID: id, AmountIDR: amount, Description: description,
			OccurredAt: occurred, OccurredAtRFC3339: occurred.Format(time.RFC3339),
		})
	}
	return mutations, nil
}

// parseAmount reads Indonesian statement formatting: 1.234.567,89 and
// 1,234,567.89 both appear, and the fractional part is discarded because rupiah
// amounts here are whole.
func parseAmount(raw string) (int64, error) {
	cleaned := strings.NewReplacer(" ", "", "Rp", "", " ", "").Replace(raw)
	if cleaned == "" {
		return 0, fmt.Errorf("nominal kosong")
	}
	// A separator is decimal only when one or two digits follow it. Exactly
	// three means thousands — "1,500,000" and "1.234.567,89" both end in a
	// separator, and treating the last one as decimal turns Rp1.500.000 into
	// Rp1.500. That amount then matches no invoice and reads as a customer who
	// never paid, which is the silent failure this parser exists to avoid.
	last := strings.LastIndexAny(cleaned, ".,")
	if last >= 0 {
		if digits := len(cleaned) - last - 1; digits >= 1 && digits <= 2 {
			cleaned = cleaned[:last]
		}
	}
	cleaned = strings.NewReplacer(".", "", ",", "").Replace(cleaned)
	value, err := strconv.ParseInt(cleaned, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("nominal %q tidak terbaca", raw)
	}
	return value, nil
}

// deriveID builds a stable id when the export carries no reference number.
//
// The honest limitation, stated because it has consequences: two genuinely
// separate transfers on the same day, for the same amount, with the same
// description are indistinguishable in such an export, and this treats them as
// one. That under-counts rather than double-settles, which is the safer
// direction — but it is a reason to prefer an export that carries reference
// numbers, and to check the unmatched queue when a travel says they paid.
func deriveID(occurred time.Time, amount int64, description string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%s",
		occurred.Format("2006-01-02"), amount, strings.ToLower(description))))
	return "derived-" + hex.EncodeToString(sum[:])[:24]
}
