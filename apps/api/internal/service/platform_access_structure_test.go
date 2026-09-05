package service

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// F4 (TUGAS-PANEL-SAAS.md, TAHAP F): "setiap RPC platform baru diuji dua
// arah." Writing a near-duplicate HTTP test for each of PlatformService's
// ~70 RPCs proving "no session -> unauthenticated, operator owner ->
// permission_denied, admin -> granted, revoked -> denied next call" would
// mostly re-prove the same fact 70 times over: every one of them delegates,
// directly or through a shared private helper, to the exact same
// requirePlatformAdmin call — and that shared function's own behavior
// (unauthenticated, denied, granted, revoked) is already proven with a real
// HTTP round trip by TestPlatformPanelIsClosedToOperatorStaffIntegration in
// platform_http_test.go.
//
// What is NOT proven by that one test is the thing most likely to actually
// break: a new RPC's handler or service method forgetting to call the guard
// at all. That is a structural property of the source code, so this test
// checks it structurally and exhaustively — every RPC listed in
// platform.proto, not a hand-picked sample — by tracing each
// PlatformService method's call graph (following calls to other
// PlatformService methods) for requirePlatformAdmin. A future RPC that
// skips the check fails this test the moment it is added, before anyone
// needs to think to write a dedicated access test for it.
func TestEveryPlatformServiceRPCRequiresPlatformAdminIntegration(t *testing.T) {
	repoRoot := findRepoRoot(t)

	protoPath := filepath.Join(repoRoot, "proto", "hajj", "v1", "platform.proto")
	protoSrc, err := os.ReadFile(protoPath)
	if err != nil {
		t.Fatalf("read platform.proto: %v", err)
	}
	serviceBlock := extractPlatformServiceBlock(t, string(protoSrc))
	rpcNames := regexp.MustCompile(`(?m)^\s*rpc (\w+)\(`).FindAllStringSubmatch(serviceBlock, -1)
	if len(rpcNames) < 40 {
		t.Fatalf("hanya menemukan %d rpc di PlatformService — pola ekstraksi mungkin salah", len(rpcNames))
	}

	serviceDir := filepath.Join(repoRoot, "apps", "api", "internal", "service")
	matches, err := filepath.Glob(filepath.Join(serviceDir, "platform*.go"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("glob platform*.go: %v (%d files)", err, len(matches))
	}
	funcBodies := map[string]string{}
	funcPattern := regexp.MustCompile(`(?m)^func \(s \*PlatformService\) (\w+)\(`)
	for _, path := range matches {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(src)
		locs := funcPattern.FindAllStringSubmatchIndex(text, -1)
		for i, loc := range locs {
			name := text[loc[2]:loc[3]]
			end := len(text)
			if i+1 < len(locs) {
				end = locs[i+1][0]
			}
			funcBodies[name] = text[loc[0]:end]
		}
	}

	callPattern := regexp.MustCompile(`s\.(\w+)\(`)
	var requiresGuard func(name string, seen map[string]bool) bool
	requiresGuard = func(name string, seen map[string]bool) bool {
		if seen[name] {
			return false
		}
		seen[name] = true
		body, ok := funcBodies[name]
		if !ok {
			return false
		}
		if regexp.MustCompile(`requirePlatformAdmin`).MatchString(body) {
			return true
		}
		for _, m := range callPattern.FindAllStringSubmatch(body, -1) {
			if requiresGuard(m[1], seen) {
				return true
			}
		}
		return false
	}

	// AmIPlatformAdmin is the one deliberate exception: it answers only
	// about the caller themselves, and its own doc comment in platform.proto
	// says so explicitly — reachable by any signed-in user by design.
	const deliberatelyPublic = "AmIPlatformAdmin"

	var unguarded []string
	for _, m := range rpcNames {
		name := m[1]
		if name == deliberatelyPublic {
			continue
		}
		if _, ok := funcBodies[name]; !ok {
			unguarded = append(unguarded, name+" (metode tidak ditemukan di service/platform*.go)")
			continue
		}
		if !requiresGuard(name, map[string]bool{}) {
			unguarded = append(unguarded, name)
		}
	}

	if len(unguarded) > 0 {
		t.Fatalf("RPC PlatformService tanpa requirePlatformAdmin di rantai panggilannya:\n%v", unguarded)
	}
}

func extractPlatformServiceBlock(t *testing.T, proto string) string {
	t.Helper()
	start := regexp.MustCompile(`(?m)^service PlatformService \{`).FindStringIndex(proto)
	if start == nil {
		t.Fatal("service PlatformService { tidak ditemukan di platform.proto")
	}
	depth := 0
	for i := start[1] - 1; i < len(proto); i++ {
		switch proto[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return proto[start[0] : i+1]
			}
		}
	}
	t.Fatal("kurung kurawal service PlatformService tidak seimbang")
	return ""
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "proto", "hajj", "v1", "platform.proto")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root (berisi proto/hajj/v1/platform.proto) tidak ditemukan dari " + dir)
		}
		dir = parent
	}
}
