package service

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/domain"
)

// Supplier cost is paid by TawafiqHub, so the platform base must cover it on
// its own. Travel and agent markups belong to other parties and cannot hide a
// loss at the platform level even when the customer-facing total is higher.
func TestBasePriceMustCoverSupplierCost(t *testing.T) {
	cost := int64(90_000)
	product := &domain.Product{SupplierCostIDR: &cost}

	t.Run("a base above cost sells", func(t *testing.T) {
		price := Price{Quantity: 1, BasePriceIDR: 100_000, TotalPriceIDR: 120_000}
		if err := ensurePriceCoversSupplierCost(product, price); err != nil {
			t.Fatalf("a profitable sale was refused: %v", err)
		}
	})

	t.Run("selling exactly at cost is allowed", func(t *testing.T) {
		price := Price{Quantity: 1, BasePriceIDR: 90_000, TotalPriceIDR: 110_000}
		if err := ensurePriceCoversSupplierCost(product, price); err != nil {
			t.Fatalf("a break-even sale was refused: %v", err)
		}
	})

	t.Run("other parties' markups cannot conceal a platform loss", func(t *testing.T) {
		price := Price{Quantity: 1, BasePriceIDR: 89_999, TotalPriceIDR: 200_000}
		if err := ensurePriceCoversSupplierCost(product, price); err == nil {
			t.Fatal("a platform base one rupiah below cost was allowed")
		}
	})

	t.Run("quantity is applied to both sides", func(t *testing.T) {
		price := Price{Quantity: 10, BasePriceIDR: 899_990, TotalPriceIDR: 2_000_000}
		if err := ensurePriceCoversSupplierCost(product, price); err == nil {
			t.Fatal("ten sales below cost were allowed")
		}
		price.BasePriceIDR = 900_000
		if err := ensurePriceCoversSupplierCost(product, price); err != nil {
			t.Fatalf("ten break-even sales were refused: %v", err)
		}
	})

	t.Run("an uncosted product is sold without inventing a floor", func(t *testing.T) {
		price := Price{Quantity: 1, BasePriceIDR: 1, TotalPriceIDR: 1}
		if err := ensurePriceCoversSupplierCost(&domain.Product{}, price); err != nil {
			t.Fatalf("an uncosted product was refused: %v", err)
		}
	})
}

func TestPilgrimOrderPricingReportsConfigurationGaps(t *testing.T) {
	product := &domain.Product{}

	_, err := pricePilgrimOrder(product, domain.PriceLevels{Configured: true}, 1, "")
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || !strings.Contains(err.Error(), ErrBasePriceUnset.Error()) {
		t.Fatalf("missing base returned %v, want an actionable failed precondition", err)
	}

	base := int64(10_000)
	_, err = pricePilgrimOrder(product, domain.PriceLevels{BasePriceIDR: &base}, 1, "")
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || !strings.Contains(err.Error(), ErrMarkupUnset.Error()) {
		t.Fatalf("missing markup returned %v, want an actionable failed precondition", err)
	}
}
