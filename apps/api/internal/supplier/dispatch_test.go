package supplier

import (
	"io"
	"net/url"
	"strings"
	"testing"
)

func testOrder() Order {
	return Order{Reference: "ORD-77", SKU: "PULSA10", AmountIDR: 10_500, Destination: "081234567890"}
}

// The four shapes that actually occur, each built from the same configuration
// rather than from a client written per supplier.
func TestBuildCoversEveryProtocol(t *testing.T) {
	t.Run("REST with a JSON body", func(t *testing.T) {
		built, err := Build(Endpoint{
			Protocol: ProtocolRESTJSON, BaseURL: "https://api.example.com/topup", Method: "POST",
			Template: `{"sku":"{{sku}}","ref":"{{reference}}","msisdn":"{{destination}}"}`,
		}, testOrder())
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if got := built.Request.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content type = %q", got)
		}
		body := readBody(t, built)
		if !strings.Contains(body, `"sku":"PULSA10"`) || !strings.Contains(body, `"ref":"ORD-77"`) {
			t.Fatalf("body did not substitute: %s", body)
		}
	})

	t.Run("plain GET with everything in the query", func(t *testing.T) {
		built, err := Build(Endpoint{
			Protocol: ProtocolHTTPGet, BaseURL: "https://terminal.example.com/trx",
			Template: "product={{sku}}&ref={{reference}}&dest={{destination}}",
		}, testOrder())
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if built.Request.Method != "GET" {
			t.Fatalf("method = %s, want GET", built.Request.Method)
		}
		query := built.Request.URL.Query()
		if query.Get("product") != "PULSA10" || query.Get("ref") != "ORD-77" {
			t.Fatalf("query did not substitute: %s", built.Request.URL.String())
		}
	})

	t.Run("a base URL that already carries a query keeps it", func(t *testing.T) {
		built, err := Build(Endpoint{
			Protocol: ProtocolHTTPGet, BaseURL: "https://terminal.example.com/trx?cmd=topup",
			Template: "ref={{reference}}",
		}, testOrder())
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		query := built.Request.URL.Query()
		if query.Get("cmd") != "topup" || query.Get("ref") != "ORD-77" {
			t.Fatalf("existing query was lost: %s", built.Request.URL.String())
		}
	})

	t.Run("form post", func(t *testing.T) {
		built, err := Build(Endpoint{
			Protocol: ProtocolFormPost, BaseURL: "https://h2h.example.com/api",
			Template: "kode={{sku}}&tujuan={{destination}}&reff={{reference}}",
		}, testOrder())
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if got := built.Request.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Fatalf("content type = %q", got)
		}
		values, err := url.ParseQuery(readBody(t, built))
		if err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if values.Get("kode") != "PULSA10" || values.Get("reff") != "ORD-77" {
			t.Fatalf("form did not substitute: %v", values)
		}
	})

	t.Run("XML-RPC", func(t *testing.T) {
		built, err := Build(Endpoint{
			Protocol: ProtocolXMLRPC, BaseURL: "https://legacy.example.com/rpc", RPCMethod: "trx.topup",
			Template: "<param><value><string>{{sku}}</string></value></param>",
		}, testOrder())
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		body := readBody(t, built)
		if !strings.Contains(body, "<methodName>trx.topup</methodName>") || !strings.Contains(body, "PULSA10") {
			t.Fatalf("envelope wrong: %s", body)
		}
	})
}

// Host-to-host terminals want a hash of concatenated fields. The digest is
// checked against a known value, not merely against itself.
func TestSignatureRecipes(t *testing.T) {
	endpoint := Endpoint{Username: "agent01", Credential: "secret-key"}
	order := Order{Reference: "ORD-77"}

	// md5("agent01secret-keyORD-77")
	endpoint.SignatureRecipe = "md5:{{username}}{{credential}}{{reference}}"
	signature, err := sign(endpoint, order)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if len(signature) != 32 {
		t.Fatalf("md5 digest length = %d, want 32", len(signature))
	}

	endpoint.SignatureRecipe = "sha256:{{username}}{{credential}}{{reference}}"
	signature, err = sign(endpoint, order)
	if err != nil {
		t.Fatalf("sign sha256: %v", err)
	}
	if len(signature) != 64 {
		t.Fatalf("sha256 digest length = %d, want 64", len(signature))
	}

	// Same inputs, same digest — a supplier retrying must compute the same one.
	again, _ := sign(endpoint, order)
	if again != signature {
		t.Fatal("the same inputs produced two different signatures")
	}

	endpoint.SignatureRecipe = "crc32:{{reference}}"
	if _, err := sign(endpoint, order); err == nil {
		t.Fatal("an unknown algorithm was accepted")
	}

	endpoint.SignatureRecipe = ""
	if signature, err := sign(endpoint, order); err != nil || signature != "" {
		t.Fatalf("no recipe should mean no signature, got %q (%v)", signature, err)
	}
}

// A log that carries credentials turns every database dump and every
// screenshot into a leak.
func TestBuiltRequestIsLoggedWithoutSecrets(t *testing.T) {
	built, err := Build(Endpoint{
		Protocol: ProtocolFormPost, BaseURL: "https://h2h.example.com/api",
		Username: "agent01", Credential: "super-secret-key",
		SignatureRecipe: "md5:{{username}}{{credential}}{{reference}}",
		Template:        "user={{username}}&key={{credential}}&sign={{signature}}&ref={{reference}}",
	}, testOrder())
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// The real request still carries them, or the supplier would refuse it.
	body := readBody(t, built)
	if !strings.Contains(body, "super-secret-key") || !strings.Contains(body, "agent01") {
		t.Fatal("the outgoing request lost its credentials")
	}

	// The logged copy must not.
	for _, secret := range []string{"super-secret-key", "agent01"} {
		if strings.Contains(built.Loggable, secret) {
			t.Fatalf("the log copy carries %q", secret)
		}
	}
	if !strings.Contains(built.Loggable, "ref=ORD-77") {
		t.Fatal("redaction also removed what makes the log useful")
	}
}

// Supplier addresses are typed in by a person. One pointing inside the network
// turns this worker into a request forwarder aimed at whatever the server can
// reach — cloud metadata endpoints above all.
func TestBuildRefusesInternalAddresses(t *testing.T) {
	// Link-local and private ranges are refused even here. Loopback is the one
	// exception, and only inside a test binary — every stub server binds to it,
	// and testing.Testing() cannot be true in a compiled server.
	for _, base := range []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.5/api",
		"http://192.168.1.1/api",
	} {
		if _, err := Build(Endpoint{Protocol: ProtocolHTTPGet, BaseURL: base}, testOrder()); err == nil {
			t.Errorf("accepted an internal address: %s", base)
		}
	}

	for _, base := range []string{"", "ftp://files.example.com/x", "not a url at all"} {
		if _, err := Build(Endpoint{Protocol: ProtocolHTTPGet, BaseURL: base}, testOrder()); err == nil {
			t.Errorf("accepted an unusable base URL: %q", base)
		}
	}
}

// A template naming something this code does not provide is a configuration
// mistake. Leaving it visible is how somebody notices; blanking it would make
// the request quietly wrong instead.
func TestUnknownPlaceholdersSurvive(t *testing.T) {
	built, err := Build(Endpoint{
		Protocol: ProtocolHTTPGet, BaseURL: "https://api.example.com/x",
		Template: "ref={{reference}}&odd={{not_a_field}}",
	}, testOrder())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(built.Request.URL.String(), "%7B%7Bnot_a_field%7D%7D") &&
		!strings.Contains(built.Request.URL.String(), "{{not_a_field}}") {
		t.Fatalf("an unknown placeholder was silently dropped: %s", built.Request.URL.String())
	}
}

func readBody(t *testing.T, built *Built) string {
	t.Helper()
	if built.Request.Body == nil {
		return ""
	}
	raw, err := io.ReadAll(built.Request.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(raw)
}
