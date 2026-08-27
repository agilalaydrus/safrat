// Package supplier turns what a provider said into what the system should do.
//
// Suppliers in this market answer in wildly different shapes — JSON, form
// bodies, plain SMS-style text — and change them without warning. Parsing is
// therefore data rather than code: ordered patterns held in the database and
// editable from the admin panel, so a supplier shifting a field is a row edit
// rather than a deploy.
package supplier

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Outcome is what a response means for the transaction behind it.
type Outcome string

const (
	// OutcomeSuccess — the supplier delivered.
	OutcomeSuccess Outcome = "SUCCESS"
	// OutcomeFailed — the supplier refused, permanently.
	OutcomeFailed Outcome = "FAILED"
	// OutcomePending — accepted, not yet delivered. A later callback decides.
	OutcomePending Outcome = "PENDING"
	// OutcomeUnmatched — no rule recognised this. Deliberately its own outcome
	// rather than being folded into FAILED: a response nobody taught the system
	// to read is a gap in the rules, and treating it as a failure would refund
	// transactions the supplier may well have delivered.
	OutcomeUnmatched Outcome = "UNMATCHED"
)

// Rule is one ordered pattern applied to a supplier's raw response.
type Rule struct {
	ID       string
	Priority int32
	Pattern  string
	Outcome  Outcome
	// ReferenceGroup and CostGroup name capture groups to lift out when the
	// pattern matches. Both optional — plenty of responses carry neither.
	ReferenceGroup string
	CostGroup      string
}

// Reading is what a response was understood to mean.
type Reading struct {
	Outcome Outcome
	// RuleID is the rule that decided, empty when nothing matched.
	RuleID string
	// Reference is the supplier's own transaction id, for tracing a dispute.
	Reference string
	// CostIDR is what they charged, when the response says. Nil when it does
	// not — which is different from zero, and must stay different, or an
	// unstated cost would look like a free product.
	CostIDR *int64
}

var errNoGroup = errors.New("capture group not present in the pattern")

// Compile validates a rule before it is ever used against a real response.
//
// Called when a rule is saved, not when it runs: a pattern that does not
// compile, or names a group it does not define, should be refused by the admin
// panel rather than silently failing over live transactions at three in the
// morning.
func Compile(rule Rule) (*regexp.Regexp, error) {
	if strings.TrimSpace(rule.Pattern) == "" {
		return nil, errors.New("pattern kosong")
	}
	compiled, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return nil, fmt.Errorf("pola tidak valid: %w", err)
	}
	names := compiled.SubexpNames()
	for _, group := range []struct{ label, name string }{
		{"reference_group", rule.ReferenceGroup},
		{"cost_group", rule.CostGroup},
	} {
		if group.name == "" {
			continue
		}
		if !containsGroup(names, group.name) {
			return nil, fmt.Errorf("%s %q: %w", group.label, group.name, errNoGroup)
		}
	}
	switch rule.Outcome {
	case OutcomeSuccess, OutcomeFailed, OutcomePending:
	default:
		return nil, fmt.Errorf("hasil %q tidak dikenal", rule.Outcome)
	}
	return compiled, nil
}

func containsGroup(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

// Read applies rules in order and returns what the first match means.
//
// Rules are expected pre-sorted by priority then age, so ordering is total and
// never depends on the order rows came back in. A rule that fails to compile is
// skipped rather than aborting the read: one bad pattern must not stop a later,
// correct one from recognising a delivered transaction.
func Read(rules []Rule, response string) Reading {
	for _, rule := range rules {
		compiled, err := Compile(rule)
		if err != nil {
			continue
		}
		match := compiled.FindStringSubmatch(response)
		if match == nil {
			continue
		}
		reading := Reading{Outcome: rule.Outcome, RuleID: rule.ID}
		names := compiled.SubexpNames()
		if rule.ReferenceGroup != "" {
			reading.Reference = capture(match, names, rule.ReferenceGroup)
		}
		if rule.CostGroup != "" {
			if cost, ok := parseAmount(capture(match, names, rule.CostGroup)); ok {
				reading.CostIDR = &cost
			}
		}
		return reading
	}
	return Reading{Outcome: OutcomeUnmatched}
}

func capture(match []string, names []string, want string) string {
	for index, name := range names {
		if name == want && index < len(match) {
			return match[index]
		}
	}
	return ""
}

// parseAmount reads an amount the way suppliers actually write them:
// "12500", "12.500", "12,500", "Rp 12.500", sometimes with a trailing ".00".
//
// Anything it cannot read confidently is refused rather than guessed at. A cost
// silently misread by a factor of a hundred would set a price floor that either
// blocks every sale or protects nothing.
func parseAmount(raw string) (int64, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	trimmed = strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(trimmed, "Rp")), " ")
	// A two-digit tail after the final separator is minor units, which IDR does
	// not use in practice — drop it rather than reading 12.500,00 as 1250000.
	if index := strings.LastIndexAny(trimmed, ".,"); index >= 0 && len(trimmed)-index == 3 {
		head, tail := trimmed[:index], trimmed[index+1:]
		if tail == "00" && strings.ContainsAny(head, ".,") {
			trimmed = head
		}
	}
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		if r == '.' || r == ',' || r == ' ' {
			return -1
		}
		return r
	}, trimmed)
	amount, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || amount < 0 {
		return 0, false
	}
	return amount, true
}
