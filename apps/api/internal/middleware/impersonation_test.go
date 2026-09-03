package middleware

import (
	"strings"
	"testing"

	_ "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// Every RPC this server exposes, checked one by one.
//
// The point is not that the current list is correct — it is that the list
// cannot quietly stop being correct. A new write RPC added next year is
// refused by default, and this test fails loudly if somebody names a write in
// a way that would slip past the read-only rule.
//
// Verbs are listed here rather than derived from the rule under test. Deriving
// them from readOnlyPrefixes would make the assertion say "the rule agrees with
// itself", which is true of any rule.
var writeVerbs = []string{
	"Create", "Update", "Delete", "Set", "Save", "Submit", "Record", "Confirm",
	"Approve", "Reject", "Assign", "Remove", "Add", "Mark", "Resolve", "Send",
	"Issue", "Apply", "Start", "End", "Cancel", "Refund", "Grant", "Revoke",
	"Publish", "Register", "Queue", "Promote", "Join", "Leave", "Move", "Bulk",
	"Allocate", "Deallocate", "Check", "Complete", "Request", "Acknowledge",
	"Invalidate", "Link", "Regenerate", "Report", "Test", "Upload", "Rotate",
	"Extend", "Suspend", "Void", "Ignore", "Settle", "Import", "Restore",
	"Reorder", "Toggle", "Enable", "Disable", "Reset", "Trigger", "Sync",
	"Transfer", "Withdraw", "Deposit", "Pay", "Charge", "Close", "Open",
}

func TestImpersonationRefusesEveryWriteProcedure(t *testing.T) {
	checked, allowed := 0, 0
	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(string(file.Package()), "hajj.v1") {
			return true
		}
		services := file.Services()
		for i := range services.Len() {
			service := services.Get(i)
			methods := service.Methods()
			for j := range methods.Len() {
				method := methods.Get(j)
				procedure := "/" + string(service.FullName()) + "/" + string(method.Name())
				checked++
				permitted := ImpersonationAllows(procedure)
				if permitted {
					allowed++
				}
				name := string(method.Name())
				for _, verb := range writeVerbs {
					if !strings.HasPrefix(name, verb) {
						continue
					}
					if permitted {
						t.Errorf("impersonasi mengizinkan %s — namanya dimulai dengan %q, jadi ini menulis", procedure, verb)
					}
				}
				// Anything the rule lets through must read like a read. This is
				// what catches a future GetOrCreate, or a write named Listen.
				if permitted && !startsWithReadVerb(name) {
					t.Errorf("impersonasi mengizinkan %s, tapi namanya bukan pembacaan", procedure)
				}
			}
		}
		return true
	})

	// A registry that matched nothing would pass every assertion above without
	// checking a single procedure.
	if checked < 100 {
		t.Fatalf("hanya %d prosedur diperiksa — deskriptor tidak termuat?", checked)
	}
	if allowed == 0 {
		t.Fatal("tidak ada satu pun prosedur yang diizinkan — impersonasi tidak akan berguna sama sekali")
	}
	t.Logf("%d prosedur diperiksa, %d boleh dibaca saat impersonasi", checked, allowed)
}

func startsWithReadVerb(method string) bool {
	for _, prefix := range []string{"List", "Get", "Preview", "Count", "Am"} {
		if strings.HasPrefix(method, prefix) {
			return true
		}
	}
	return false
}

// The platform's own surface is closed outright. Reading it while wearing a
// customer's face would put every other tenant's data on that customer's
// screen, and the admin already has it through their own session.
func TestImpersonationClosesThePlatformSurface(t *testing.T) {
	for _, procedure := range []string{
		"/hajj.v1.PlatformService/ListOperators",
		"/hajj.v1.PlatformService/GetTenantDetail",
		"/hajj.v1.PlatformService/AmIPlatformAdmin",
		"/hajj.v1.PlatformService/ListUsage",
		"/hajj.v1.FunnelService/RecordEvent",
	} {
		if ImpersonationAllows(procedure) {
			t.Errorf("%s terbuka untuk sesi impersonasi", procedure)
		}
	}
}

func TestImpersonationAllowsOrdinaryReads(t *testing.T) {
	for _, procedure := range []string{
		"/hajj.v1.PilgrimService/ListPilgrims",
		"/hajj.v1.SeasonService/GetSeasonAnalytics",
		"/hajj.v1.OrderService/ListOrders",
	} {
		if !ImpersonationAllows(procedure) {
			t.Errorf("%s tertutup, padahal ini yang membuat impersonasi berguna", procedure)
		}
	}
	// A name that starts with a read verb but continues into a write.
	if ImpersonationAllows("/hajj.v1.PilgrimService/GetOrCreatePilgrim") {
		t.Error("GetOrCreate lolos — awalan pembacaan tidak cukup, isinya menulis")
	}
	// Malformed input must not fall through to allowed.
	for _, procedure := range []string{"", "ListPilgrims", "/", "/hajj.v1.PilgrimService/"} {
		if ImpersonationAllows(procedure) {
			t.Errorf("prosedur tidak berbentuk %q diizinkan", procedure)
		}
	}
}
