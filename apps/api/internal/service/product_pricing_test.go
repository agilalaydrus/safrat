package service

import (
	"strings"
	"testing"

	"github.com/hajj-saas/api/internal/domain"
)

func digitalProduct(supplierCost *int64) *domain.Product {
	return &domain.Product{
		ID: "11111111-1111-1111-1111-111111111111", Name: "Pulsa Telkomsel 10K",
		Code: "PULSA-TSEL-10K", Category: "PPOB_CREDIT", SupplierCostIDR: supplierCost,
	}
}

// The whole point of computing sellability through the real gate: whatever the
// screen says is sellable must actually sell, and whatever it refuses must
// refuse at checkout for the same reason. A screen with its own opinion drifts,
// and the drift only shows up when a customer is already trying to pay.
func TestPricingScreenAgreesWithCheckout(t *testing.T) {
	cost := int64(9_500)
	cases := []struct {
		name   string
		levels domain.PriceLevels
	}{
		{"lengkap", levels(10_000, 500, 300)},
		{"markup nol", levels(10_000, 0, 0)},
		{"di bawah harga modal", levels(9_000, 100, 100)},
		{"markup belum diatur", domain.PriceLevels{BasePriceIDR: idr(10_000)}},
		{"harga dasar kosong", domain.PriceLevels{OperatorMarkupIDR: 500, Configured: true}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			product := digitalProduct(&cost)
			msg := pricingMessage(product, c.levels)

			_, checkoutErr := pricePilgrimOrder(product, c.levels, 1, "")

			if msg.Sellable != (checkoutErr == nil) {
				t.Fatalf("layar bilang sellable=%v tapi checkout err=%v", msg.Sellable, checkoutErr)
			}
			if !msg.Sellable && strings.TrimSpace(msg.UnsellableReason) == "" {
				t.Fatal("produk ditolak tanpa alasan yang bisa dibaca orang")
			}
			if msg.Sellable && msg.UnsellableReason != "" {
				t.Fatalf("produk bisa dijual tapi membawa alasan penolakan: %q", msg.UnsellableReason)
			}
		})
	}
}

func TestPricingScreenShowsBothBuyerPrices(t *testing.T) {
	msg := pricingMessage(digitalProduct(nil), levels(10_000, 500, 300))

	if !msg.Sellable {
		t.Fatalf("harus bisa dijual: %s", msg.UnsellableReason)
	}
	if msg.AgentPriceIdr != 10_500 {
		t.Fatalf("harga agen = %d, mau 10500", msg.AgentPriceIdr)
	}
	if msg.PilgrimPriceIdr != 10_800 {
		t.Fatalf("harga jamaah = %d, mau 10800", msg.PilgrimPriceIdr)
	}
	// The parts must add back to the whole, or the screen is explaining a
	// price that is not the one being charged.
	if msg.BasePriceIdr+msg.OperatorMarkupIdr+msg.AgentMarkupIdr != msg.PilgrimPriceIdr {
		t.Fatal("komponen tidak menjumlah ke harga jamaah")
	}
	if msg.BasePriceIdr+msg.OperatorMarkupIdr != msg.AgentPriceIdr {
		t.Fatal("komponen tidak menjumlah ke harga agen")
	}
}

// Zero markup and absent markup produce the same numbers, so only these flags
// can tell them apart. Getting this wrong means an unconfigured product looks
// deliberately priced at cost.
func TestUnconfiguredIsDistinguishableFromZero(t *testing.T) {
	zero := pricingMessage(digitalProduct(nil), levels(10_000, 0, 0))
	if !zero.MarkupConfigured || !zero.Sellable {
		t.Fatal("markup nol adalah keputusan yang sah dan harus bisa dijual")
	}

	absent := pricingMessage(digitalProduct(nil), domain.PriceLevels{BasePriceIDR: idr(10_000)})
	if absent.MarkupConfigured {
		t.Fatal("produk tanpa baris markup tidak boleh dilaporkan sudah dikonfigurasi")
	}
	if absent.Sellable {
		t.Fatal("produk yang belum dikonfigurasi tidak boleh bisa dijual")
	}

	unsetBase := pricingMessage(digitalProduct(nil), domain.PriceLevels{OperatorMarkupIDR: 500, Configured: true})
	if unsetBase.BasePriceSet {
		t.Fatal("harga dasar kosong tidak boleh dilaporkan sudah diatur")
	}
	if unsetBase.BasePriceIdr != 0 || unsetBase.Sellable {
		t.Fatal("harga dasar kosong harus nol dan tidak bisa dijual")
	}
}
