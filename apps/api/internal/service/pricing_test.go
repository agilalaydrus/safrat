package service

import (
	"errors"
	"testing"

	"github.com/hajj-saas/api/internal/domain"
)

func idr(v int64) *int64 { return &v }

func levels(base int64, operator, agent int64) domain.PriceLevels {
	return domain.PriceLevels{
		BasePriceIDR:      idr(base),
		OperatorMarkupIDR: operator,
		AgentMarkupIDR:    agent,
		Configured:        true,
	}
}

func TestJamaahPaysEveryLevel(t *testing.T) {
	p, err := ComputePrice(levels(10_000, 500, 300), 1, BuyerPilgrim, "agent-1")
	if err != nil {
		t.Fatalf("tak terduga: %v", err)
	}
	if p.TotalPriceIDR != 10_800 {
		t.Fatalf("total = %d, mau 10800", p.TotalPriceIDR)
	}
	if p.AgentCommissionIDR != 300 {
		t.Fatalf("komisi = %d, mau 300", p.AgentCommissionIDR)
	}
}

// The agent price is the jamaah price minus the agent level — not a discount
// applied afterwards, which would be a different number the moment anything
// else changed.
func TestAgentBuyingForThemselvesPaysNoAgentMarkup(t *testing.T) {
	p, err := ComputePrice(levels(10_000, 500, 300), 1, BuyerAgent, "agent-1")
	if err != nil {
		t.Fatalf("tak terduga: %v", err)
	}
	if p.TotalPriceIDR != 10_500 {
		t.Fatalf("total = %d, mau 10500", p.TotalPriceIDR)
	}
	if p.AgentMarkupIDR != 0 || p.AgentCommissionIDR != 0 {
		t.Fatalf("agen membeli untuk diri sendiri tidak boleh menghasilkan komisi: markup=%d komisi=%d",
			p.AgentMarkupIDR, p.AgentCommissionIDR)
	}
}

// The referrer is passed even here: an agent buying on their own account has
// a referrer of their own, and that must not turn into a payout either.
func TestAgentBuyerEarnsNothingEvenWithAReferrer(t *testing.T) {
	p, _ := ComputePrice(levels(10_000, 500, 300), 3, BuyerAgent, "upline-agent")
	if p.AgentCommissionIDR != 0 {
		t.Fatalf("komisi = %d, mau 0", p.AgentCommissionIDR)
	}
}

// A jamaah with no referrer must pay the same as one with a referrer, or
// agents would be quoting their own customers a worse price than a walk-in.
func TestUnreferredJamaahPaysTheSamePrice(t *testing.T) {
	referred, _ := ComputePrice(levels(10_000, 500, 300), 1, BuyerPilgrim, "agent-1")
	walkIn, _ := ComputePrice(levels(10_000, 500, 300), 1, BuyerPilgrim, "")

	if referred.TotalPriceIDR != walkIn.TotalPriceIDR {
		t.Fatalf("harga berbeda: dirujuk=%d langsung=%d", referred.TotalPriceIDR, walkIn.TotalPriceIDR)
	}
	// The money still has to land somewhere.
	if walkIn.AgentCommissionIDR != 0 {
		t.Fatalf("tanpa perujuk tidak boleh ada komisi, dapat %d", walkIn.AgentCommissionIDR)
	}
	if walkIn.OperatorAmountIDR != 800 {
		t.Fatalf("level agen yang tak terpakai harus jatuh ke travel: dapat %d, mau 800", walkIn.OperatorAmountIDR)
	}
}

// Zero markup is a decision an operator is allowed to make; an unset product
// is a gap. They price identically and must not be confused.
func TestZeroMarkupSellsAtBaseButUnconfiguredIsRefused(t *testing.T) {
	p, err := ComputePrice(levels(10_000, 0, 0), 1, BuyerPilgrim, "")
	if err != nil {
		t.Fatalf("markup nol adalah pengaturan yang sah: %v", err)
	}
	if p.TotalPriceIDR != 10_000 {
		t.Fatalf("total = %d, mau 10000", p.TotalPriceIDR)
	}

	unconfigured := domain.PriceLevels{BasePriceIDR: idr(10_000)}
	if _, err := ComputePrice(unconfigured, 1, BuyerPilgrim, ""); !errors.Is(err, ErrMarkupUnset) {
		t.Fatalf("produk belum dikonfigurasi harus ditolak, dapat %v", err)
	}
}

func TestMissingBaseIsRefusedNotTreatedAsFree(t *testing.T) {
	l := levels(0, 500, 300)
	l.BasePriceIDR = nil
	if _, err := ComputePrice(l, 1, BuyerPilgrim, ""); !errors.Is(err, ErrBasePriceUnset) {
		t.Fatalf("harga dasar kosong harus ditolak, dapat %v", err)
	}
}

// The property that makes this scheme worth the migration: because the price
// is summed rather than divided, nothing is ever lost to rounding. The old
// basis-point split dropped a rupiah on roughly one order in two hundred.
func TestNothingIsEverLostToRounding(t *testing.T) {
	awkward := []struct{ base, operator, agent int64 }{
		{10_001, 333, 167}, {7, 1, 1}, {999_983, 77_777, 33_333},
		{1, 0, 0}, {123_457, 1, 99_999},
	}
	for _, c := range awkward {
		for _, qty := range []int32{1, 3, 7, 999} {
			for _, buyer := range []BuyerKind{BuyerPilgrim, BuyerAgent} {
				p, err := ComputePrice(levels(c.base, c.operator, c.agent), qty, buyer, "agent-1")
				if err != nil {
					t.Fatalf("tak terduga: %v", err)
				}

				components := p.BasePriceIDR + p.OperatorMarkupIDR + p.AgentMarkupIDR
				if components != p.TotalPriceIDR {
					t.Fatalf("komponen %d != total %d (%v qty=%d %s)",
						components, p.TotalPriceIDR, c, qty, buyer)
				}

				settled := p.PlatformAmountIDR + p.OperatorAmountIDR + p.AgentCommissionIDR
				if settled != p.TotalPriceIDR {
					t.Fatalf("penyelesaian %d != total %d (%v qty=%d %s) — ada rupiah yang hilang",
						settled, p.TotalPriceIDR, c, qty, buyer)
				}
			}
		}
	}
}

func TestQuantityMultipliesEveryLevel(t *testing.T) {
	one, _ := ComputePrice(levels(10_000, 500, 300), 1, BuyerPilgrim, "agent-1")
	ten, _ := ComputePrice(levels(10_000, 500, 300), 10, BuyerPilgrim, "agent-1")

	if ten.TotalPriceIDR != one.TotalPriceIDR*10 {
		t.Fatalf("total qty=10 adalah %d, mau %d", ten.TotalPriceIDR, one.TotalPriceIDR*10)
	}
	if ten.AgentCommissionIDR != one.AgentCommissionIDR*10 {
		t.Fatalf("komisi tidak menskala dengan kuantitas: %d vs %d",
			ten.AgentCommissionIDR, one.AgentCommissionIDR*10)
	}
}

func TestQuantityBelowOneIsRefused(t *testing.T) {
	if _, err := ComputePrice(levels(10_000, 500, 300), 0, BuyerPilgrim, ""); err == nil {
		t.Fatal("kuantitas 0 harus ditolak")
	}
}
