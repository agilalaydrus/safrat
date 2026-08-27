package supplier

import "testing"

func rules() []Rule {
	// Ordered as the database returns them: priority, then age.
	return []Rule{
		{
			ID: "rule-success", Priority: 10,
			Pattern:        `(?i)"status"\s*:\s*"?(?:success|sukses)"?.*?"trx_id"\s*:\s*"(?P<ref>[A-Z0-9-]+)".*?"price"\s*:\s*"?(?P<cost>[0-9.,]+)"?`,
			Outcome:        OutcomeSuccess,
			ReferenceGroup: "ref", CostGroup: "cost",
		},
		{
			ID: "rule-sms", Priority: 20,
			Pattern:        `(?i)^BERHASIL\.\s+SN[:=]\s*(?P<ref>\S+)\s+HARGA\s+(?P<cost>[0-9.,]+)`,
			Outcome:        OutcomeSuccess,
			ReferenceGroup: "ref", CostGroup: "cost",
		},
		{ID: "rule-pending", Priority: 30, Pattern: `(?i)(pending|diproses|sedang diproses)`, Outcome: OutcomePending},
		{ID: "rule-failed", Priority: 40, Pattern: `(?i)(gagal|failed|saldo tidak cukup|invalid)`, Outcome: OutcomeFailed},
	}
}

func TestReadRecognisesDifferentSupplierShapes(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		response  string
		outcome   Outcome
		ruleID    string
		reference string
		cost      int64
		hasCost   bool
	}{
		{
			name:     "json",
			response: `{"status":"SUCCESS","trx_id":"TRX-99812","price":"12.500"}`,
			outcome:  OutcomeSuccess, ruleID: "rule-success", reference: "TRX-99812", cost: 12_500, hasCost: true,
		},
		{
			// The same supplier, a month later, quoting nothing and lowercasing.
			name:     "json shape shifted",
			response: `{"status":success,"trx_id":"TRX-1","price":9500}`,
			outcome:  OutcomeSuccess, ruleID: "rule-success", reference: "TRX-1", cost: 9_500, hasCost: true,
		},
		{
			name:     "plain text",
			response: "BERHASIL. SN: 0012938471923 HARGA 20.000 SISA SALDO 1.980.000",
			outcome:  OutcomeSuccess, ruleID: "rule-sms", reference: "0012938471923", cost: 20_000, hasCost: true,
		},
		{
			name:     "pending wins over nothing",
			response: "Transaksi sedang diproses, mohon tunggu",
			outcome:  OutcomePending, ruleID: "rule-pending",
		},
		{
			name:     "failure",
			response: "GAGAL: saldo tidak cukup",
			outcome:  OutcomeFailed, ruleID: "rule-failed",
		},
		{
			// The case that must never be read as failure: a supplier saying
			// something nobody taught the system to read. Refunding these would
			// give money back for transactions they may well have delivered.
			name:     "unrecognised",
			response: "OK 4711",
			outcome:  OutcomeUnmatched,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			reading := Read(rules(), testCase.response)
			if reading.Outcome != testCase.outcome {
				t.Fatalf("outcome = %s, want %s", reading.Outcome, testCase.outcome)
			}
			if reading.RuleID != testCase.ruleID {
				t.Fatalf("rule = %q, want %q", reading.RuleID, testCase.ruleID)
			}
			if reading.Reference != testCase.reference {
				t.Fatalf("reference = %q, want %q", reading.Reference, testCase.reference)
			}
			if testCase.hasCost {
				if reading.CostIDR == nil || *reading.CostIDR != testCase.cost {
					t.Fatalf("cost = %v, want %d", reading.CostIDR, testCase.cost)
				}
			} else if reading.CostIDR != nil {
				t.Fatalf("cost = %d, want none reported", *reading.CostIDR)
			}
		})
	}
}

// Priority decides, not the order somebody happened to write the rules in. A
// response containing both "berhasil" and "gagal" — a supplier reporting a
// success after an earlier failure — must read as the higher-priority rule.
func TestReadHonoursPriorityOrder(t *testing.T) {
	response := "BERHASIL. SN: SN-1 HARGA 5.000 (percobaan sebelumnya gagal)"
	if reading := Read(rules(), response); reading.Outcome != OutcomeSuccess {
		t.Fatalf("outcome = %s, want SUCCESS — the higher-priority rule should decide", reading.Outcome)
	}
}

// One unusable pattern must not stop a later, correct rule from recognising a
// delivered transaction.
func TestReadSkipsRulesThatDoNotCompile(t *testing.T) {
	broken := append([]Rule{{ID: "broken", Priority: 1, Pattern: `(unclosed`, Outcome: OutcomeFailed}}, rules()...)
	reading := Read(broken, `{"status":"SUCCESS","trx_id":"TRX-7","price":"1.000"}`)
	if reading.Outcome != OutcomeSuccess || reading.RuleID != "rule-success" {
		t.Fatalf("outcome=%s rule=%s — a broken pattern shadowed a working one", reading.Outcome, reading.RuleID)
	}
}

// Rules are validated when saved, so a bad one is refused in the admin panel
// rather than discovered over live transactions.
func TestCompileRejectsRulesThatWouldFailLater(t *testing.T) {
	for _, testCase := range []struct {
		name string
		rule Rule
	}{
		{"empty pattern", Rule{Pattern: "  ", Outcome: OutcomeSuccess}},
		{"malformed pattern", Rule{Pattern: `(?P<ref>`, Outcome: OutcomeSuccess}},
		{"names a group it never defines", Rule{Pattern: `ok`, Outcome: OutcomeSuccess, ReferenceGroup: "ref"}},
		{"names a cost group it never defines", Rule{Pattern: `(?P<ref>ok)`, Outcome: OutcomeSuccess, ReferenceGroup: "ref", CostGroup: "cost"}},
		{"unknown outcome", Rule{Pattern: `ok`, Outcome: "MAYBE"}},
		{"cannot declare UNMATCHED", Rule{Pattern: `ok`, Outcome: OutcomeUnmatched}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := Compile(testCase.rule); err == nil {
				t.Fatal("accepted a rule that would fail later")
			}
		})
	}

	if _, err := Compile(Rule{Pattern: `(?P<ref>\w+)`, Outcome: OutcomeSuccess, ReferenceGroup: "ref"}); err != nil {
		t.Fatalf("rejected a valid rule: %v", err)
	}
}

// An amount misread by a factor of a hundred would set a price floor that
// either blocks every sale or protects nothing, so anything ambiguous is
// refused rather than guessed.
func TestParseAmount(t *testing.T) {
	for _, testCase := range []struct {
		raw    string
		want   int64
		usable bool
	}{
		{"12500", 12_500, true},
		{"12.500", 12_500, true},
		{"12,500", 12_500, true},
		{"Rp 12.500", 12_500, true},
		{"1.980.000", 1_980_000, true},
		// Minor units a supplier tacked on. IDR does not use them in practice.
		{"12.500,00", 12_500, true},
		{"0", 0, true},
		{"", 0, false},
		{"gratis", 0, false},
		{"-500", 0, false},
	} {
		got, ok := parseAmount(testCase.raw)
		if ok != testCase.usable {
			t.Errorf("parseAmount(%q) usable = %v, want %v", testCase.raw, ok, testCase.usable)
			continue
		}
		if ok && got != testCase.want {
			t.Errorf("parseAmount(%q) = %d, want %d", testCase.raw, got, testCase.want)
		}
	}
}
