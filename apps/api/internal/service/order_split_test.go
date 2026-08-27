package service

import (
	"testing"

	"github.com/hajj-saas/api/internal/domain"
)

// The split is integer arithmetic end to end. It used to multiply by a
// float64 fraction: 0.70 has no exact binary representation, so the product
// landed a hair under the true value and truncation dropped a rupiah. Measured
// across 25,070 price/margin combinations, 120 came out one short — always
// downward, and always out of the operator's share.
func TestComputeSplitIsExact(t *testing.T) {
	product := &domain.Product{
		PriceIDR: 699_110, PlatformMarginBps: 1500, OperatorMarginBps: 7000, AgentMarginBps: 1500,
	}

	split := computeSplit(product, 1, "agent-1")

	// 699110 * 7000 / 10000 = 489377 exactly. The float version returned
	// 489376 for this very price.
	if split.OperatorAmount != 489_377 {
		t.Fatalf("operator amount = %d, want 489377", split.OperatorAmount)
	}
	if split.PlatformAmount != 104_866 || split.AgentCommission != 104_866 {
		t.Fatalf("platform=%d agent=%d, want 104866 each", split.PlatformAmount, split.AgentCommission)
	}
	if split.TotalPrice != 699_110 {
		t.Fatalf("total = %d, want 699110", split.TotalPrice)
	}
}

// Whatever else changes, the parts may never add up to more than was charged.
func TestComputeSplitNeverExceedsTheTotal(t *testing.T) {
	for _, margins := range [][3]int32{
		{1500, 7000, 1500}, // the default, summing to exactly 100%
		{3333, 3333, 3334},
		{1, 1, 1},
		{0, 10000, 0},
		{2500, 2500, 2500}, // sums to 75%, the rest stays with the operator
	} {
		product := &domain.Product{
			PriceIDR: 1, PlatformMarginBps: margins[0], OperatorMarginBps: margins[1], AgentMarginBps: margins[2],
		}
		for _, price := range []int64{1, 7, 999, 1_000_000, 12_345_679, 999_999_999} {
			product.PriceIDR = price
			for _, quantity := range []int32{1, 3, 20} {
				split := computeSplit(product, quantity, "agent-1")
				parts := split.PlatformAmount + split.OperatorAmount + split.AgentCommission
				if parts > split.TotalPrice {
					t.Fatalf("margins %v, price %d x%d: parts total %d exceed the charge of %d",
						margins, price, quantity, parts, split.TotalPrice)
				}
				if split.PlatformAmount < 0 || split.OperatorAmount < 0 || split.AgentCommission < 0 {
					t.Fatalf("margins %v, price %d: a negative share", margins, price)
				}
			}
		}
	}
}

// Commission is zero without a referral, whatever the product's agent margin
// says (CODEX_SPEC §7).
func TestComputeSplitPaysNoCommissionWithoutAReferral(t *testing.T) {
	product := &domain.Product{
		PriceIDR: 1_000_000, PlatformMarginBps: 1500, OperatorMarginBps: 7000, AgentMarginBps: 1500,
	}
	if split := computeSplit(product, 1, ""); split.AgentCommission != 0 {
		t.Fatalf("commission = %d with no referring agent, want 0", split.AgentCommission)
	}
	if split := computeSplit(product, 1, "   "); split.AgentCommission != 0 {
		t.Fatalf("commission = %d for a blank agent id, want 0", split.AgentCommission)
	}
}
