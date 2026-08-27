package service

import (
	"strings"
	"testing"

	"github.com/hajj-saas/api/internal/domain"
)

// readyRoute is a digital product that can reach an active supplier. Digital
// categories require routing, so without this every pricing test would be
// asserting the routing refusal instead of what it means to assert.
func readyRoute() domain.RouteReadiness {
	return domain.RouteReadiness{Required: true, Exists: true, Active: true, SupplierStatus: "ACTIVE"}
}

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
			msg := pricingMessage(product, c.levels, readyRoute())

			_, checkoutErr := pricePilgrimOrder(product, c.levels, readyRoute(), 1, "")

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
	msg := pricingMessage(digitalProduct(nil), levels(10_000, 500, 300), readyRoute())

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
	zero := pricingMessage(digitalProduct(nil), levels(10_000, 0, 0), readyRoute())
	if !zero.MarkupConfigured || !zero.Sellable {
		t.Fatal("markup nol adalah keputusan yang sah dan harus bisa dijual")
	}

	absent := pricingMessage(digitalProduct(nil), domain.PriceLevels{BasePriceIDR: idr(10_000)}, readyRoute())
	if absent.MarkupConfigured {
		t.Fatal("produk tanpa baris markup tidak boleh dilaporkan sudah dikonfigurasi")
	}
	if absent.Sellable {
		t.Fatal("produk yang belum dikonfigurasi tidak boleh bisa dijual")
	}

	unsetBase := pricingMessage(digitalProduct(nil), domain.PriceLevels{OperatorMarkupIDR: 500, Configured: true}, readyRoute())
	if unsetBase.BasePriceSet {
		t.Fatal("harga dasar kosong tidak boleh dilaporkan sudah diatur")
	}
	if unsetBase.BasePriceIdr != 0 || unsetBase.Sellable {
		t.Fatal("harga dasar kosong harus nol dan tidak bisa dijual")
	}
}

// Each refusal names what is actually wrong, because each is fixed by a
// different person: routing is set by the platform, activation by whoever
// disabled it, supplier status by whoever suspended the supplier. A single
// "tidak bisa dijual" sends all three looking in the wrong place.
func TestRoutingRefusalsSayWhichThingIsWrong(t *testing.T) {
	cases := []struct {
		name   string
		route  domain.RouteReadiness
		expect string
	}{
		{"tanpa routing", domain.RouteReadiness{Required: true}, "belum diatur routing"},
		{"routing nonaktif", domain.RouteReadiness{Required: true, Exists: true, SupplierStatus: "ACTIVE"}, "dinonaktifkan"},
		{"supplier nonaktif", domain.RouteReadiness{Required: true, Exists: true, Active: true, SupplierStatus: "SUSPENDED"}, "supplier"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			msg := pricingMessage(digitalProduct(nil), levels(10_000, 500, 300), c.route)
			if msg.Sellable {
				t.Fatal("produk tanpa jalur ke supplier tidak boleh bisa dijual")
			}
			if !strings.Contains(msg.UnsellableReason, c.expect) {
				t.Fatalf("alasan %q tidak menyebut %q", msg.UnsellableReason, c.expect)
			}
		})
	}
}

// A travel package has no supplier to route to, so absent routing is not a
// fault. Getting this wrong would make every package in the catalogue
// unsellable the moment the gate was added.
func TestNonDigitalProductsNeedNoRoute(t *testing.T) {
	pkg := &domain.Product{ID: "x", Name: "Umrah 9 Hari", Category: "TRAVEL_PACKAGE"}
	msg := pricingMessage(pkg, levels(25_000_000, 1_000_000, 500_000), domain.RouteReadiness{})

	if !msg.Sellable {
		t.Fatalf("paket perjalanan ditolak karena routing: %s", msg.UnsellableReason)
	}

	// Equipment is delivered by hand too — it opens a fulfilment, but there is
	// no supplier call to route.
	kit := &domain.Product{ID: "y", Name: "Koper", Category: "EQUIPMENT"}
	if msg := pricingMessage(kit, levels(500_000, 50_000, 0), domain.RouteReadiness{}); !msg.Sellable {
		t.Fatalf("perlengkapan ditolak karena routing: %s", msg.UnsellableReason)
	}
}

// The refusal must land before a payment exists. Taking money for something
// nothing can deliver leaves a jamaah holding a paid order and someone
// unwinding it by hand.
func TestCheckoutRefusesBeforePricingSucceeds(t *testing.T) {
	product := digitalProduct(nil)
	unrouted := domain.RouteReadiness{Required: true}

	if _, err := pricePilgrimOrder(product, levels(10_000, 500, 300), unrouted, 1, ""); err == nil {
		t.Fatal("checkout jamaah menerima produk tanpa routing")
	}
	if _, err := priceAgentOrder(product, levels(10_000, 500, 300), unrouted, 1); err == nil {
		t.Fatal("checkout agen menerima produk tanpa routing")
	}
}

// Routing is platform-owned, so a travel reading these cannot fix any of them
// in their own dashboard. Every refusal has to say who to ask, or it is a dead
// end dressed as an explanation.
func TestRoutingRefusalsTellTheReaderWhoToAsk(t *testing.T) {
	unready := []domain.RouteReadiness{
		{Required: true},
		{Required: true, Exists: true, SupplierStatus: "ACTIVE"},
		{Required: true, Exists: true, Active: true, SupplierStatus: "SUSPENDED"},
	}
	for _, route := range unready {
		reason := route.Refusal()
		if reason == "" {
			t.Fatalf("%+v seharusnya ditolak", route)
		}
		if !strings.Contains(reason, "TawafiqHub") {
			t.Fatalf("alasan %q tidak memberi tahu pembaca harus menghubungi siapa", reason)
		}
	}
}
