package service

import (
	"errors"
	"strings"
)

// BuyerKind is who a transaction is for. It decides the price, not just the
// label on the record: an agent and a jamaah are quoted different numbers for
// the same product.
type BuyerKind string

const (
	BuyerPilgrim BuyerKind = "PILGRIM"
	BuyerAgent   BuyerKind = "AGENT"
)

var (
	// ErrBasePriceUnset refuses the sale rather than falling back to zero.
	// A product with no base has no price, and selling it for the markup
	// alone would hand over the platform's entire cost.
	ErrBasePriceUnset = errors.New("harga dasar produk belum diatur")

	// ErrMarkupUnset separates "nobody has configured this product for this
	// operator" from "the operator deliberately set zero markup". Both would
	// price identically, but only the first is a mistake, and treating a gap
	// as a decision is how a product ends up sold at cost by accident.
	ErrMarkupUnset = errors.New("markup produk belum diatur untuk travel ini")
)

// PriceLevels is the per-unit price built from the bottom up. Every level is
// present on every product; a level nobody has touched is zero, and zero is a
// real setting rather than a missing one.
type PriceLevels struct {
	// BasePriceIDR is what TawafiqHub charges the travel. Nil means unset,
	// which is refused — distinct from zero, which would mean free.
	BasePriceIDR *int64

	// OperatorMarkupIDR is what the travel adds. Base + this is the agent
	// price.
	OperatorMarkupIDR int64

	// AgentMarkupIDR is the agent level. Part of the jamaah price whether or
	// not a referrer exists, so a referred jamaah is never quoted more than a
	// walk-in for the same product.
	AgentMarkupIDR int64

	// Configured records that a markup row was actually found. Without it the
	// two zero markups above are indistinguishable from an unconfigured
	// product.
	Configured bool
}

// Price is one transaction's money, itemised. The three components always sum
// to the total and the three settlement amounts always sum to the total —
// checked in tests, and true by construction because nothing here divides.
type Price struct {
	Quantity int32

	// What the buyer pays, and what it is made of.
	BasePriceIDR      int64
	OperatorMarkupIDR int64
	AgentMarkupIDR    int64
	TotalPriceIDR     int64

	// Where it goes.
	PlatformAmountIDR  int64
	OperatorAmountIDR  int64
	AgentCommissionIDR int64
}

// ComputePrice builds the price for one buyer from the levels beneath them.
//
// This is addition only. The previous scheme took percentages out of a price
// somebody typed, which meant rounding: basis points made it exact to the
// rupiah but still had to round somewhere, and the remainder had to be given
// to a level by fiat. Building the price upward removes the question — every
// level's share is the number that level set, and the total is their sum.
//
// referrerAgentID is the *referrer*, never whoever placed the order. An agent
// selling to a jamaah somebody else referred must not collect that referrer's
// commission (CODEX_SPEC §7).
func ComputePrice(levels PriceLevels, quantity int32, buyer BuyerKind, referrerAgentID string) (Price, error) {
	if quantity < 1 {
		return Price{}, errors.New("kuantitas harus minimal 1")
	}
	if levels.BasePriceIDR == nil {
		return Price{}, ErrBasePriceUnset
	}
	if !levels.Configured {
		return Price{}, ErrMarkupUnset
	}

	qty := int64(quantity)
	p := Price{
		Quantity:          quantity,
		BasePriceIDR:      *levels.BasePriceIDR * qty,
		OperatorMarkupIDR: levels.OperatorMarkupIDR * qty,
	}

	// An agent buys at the agent price, which is base plus the operator's
	// markup and nothing above it. The agent level is not discounted away for
	// them — it is simply not part of what the agent price means.
	if buyer == BuyerPilgrim {
		p.AgentMarkupIDR = levels.AgentMarkupIDR * qty
	}

	p.TotalPriceIDR = p.BasePriceIDR + p.OperatorMarkupIDR + p.AgentMarkupIDR

	// The base is what TawafiqHub receives. What TawafiqHub *keeps* is that
	// minus the supplier's cost, which lives with the supplier record — the
	// platform's own margin is not this number and must not be read as it.
	p.PlatformAmountIDR = p.BasePriceIDR
	p.OperatorAmountIDR = p.OperatorMarkupIDR

	// The agent level is collected either way. With a referrer it is their
	// commission; without one it falls to the operator, because the jamaah has
	// already paid it and money that belongs to nobody cannot be left
	// unassigned.
	if p.AgentMarkupIDR > 0 && strings.TrimSpace(referrerAgentID) != "" {
		p.AgentCommissionIDR = p.AgentMarkupIDR
	} else {
		p.OperatorAmountIDR += p.AgentMarkupIDR
	}

	return p, nil
}
