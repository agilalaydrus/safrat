package supplier

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Protocol is the shape a supplier expects to be called in.
//
// These do not converge in this market and there is no prospect of them doing
// so: newer providers are REST with a JSON body, plenty are a plain GET with
// everything in the query string, and the older host-to-host terminals are
// XML-RPC. Writing a client per supplier would mean a deploy every time one is
// added, so the request is configuration, exactly like the response rules.
type Protocol string

const (
	ProtocolRESTJSON Protocol = "REST_JSON"
	ProtocolHTTPGet  Protocol = "HTTP_GET"
	ProtocolFormPost Protocol = "FORM_POST"
	ProtocolXMLRPC   Protocol = "XML_RPC"
)

// Endpoint is everything needed to call one supplier, resolved from their row
// plus the credentials their environment variables hold.
type Endpoint struct {
	Protocol Protocol
	BaseURL  string
	Method   string
	// Template is the body, or the query string for HTTP_GET. Placeholders are
	// substituted at send time.
	Template string
	// RPCMethod names the procedure for XML_RPC; ignored otherwise.
	RPCMethod string
	// Username and Credential come from environment variables, never from the
	// database. A dump therefore carries no secrets and rotating a key touches
	// no row.
	Username   string
	Credential string
	// SignatureRecipe is "algorithm:template", e.g.
	// "md5:{{username}}{{credential}}{{reference}}". The result substitutes
	// {{signature}} in Template.
	SignatureRecipe string
	Timeout         time.Duration
}

// Order is the transaction being sent.
type Order struct {
	// Reference is our own order id. It is what makes a retry the same purchase
	// to the supplier rather than a second one.
	Reference   string
	SKU         string
	AmountIDR   int64
	Destination string
}

// Built is a prepared request, with a redacted copy for the log.
type Built struct {
	Request *http.Request
	// Loggable is the request as it should be recorded: identical except that
	// credentials are replaced. A log that carries them turns every database
	// dump and every screenshot into a credential leak.
	Loggable string
	Endpoint string
}

var (
	errNoBaseURL      = errors.New("supplier belum punya base URL")
	errBadSignature   = errors.New("resep tanda tangan tidak dikenali")
	errPrivateAddress = errors.New("alamat supplier mengarah ke jaringan internal")
)

// Build prepares the outbound call.
//
// The URL is checked before anything is sent. Supplier addresses are typed in
// by a platform admin, and an address pointing at loopback or a private range
// turns this worker into a way to reach things it should never reach — cloud
// metadata endpoints most of all. Refusing it here costs nothing; discovering
// it later costs a great deal.
func Build(endpoint Endpoint, order Order) (*Built, error) {
	base := strings.TrimSpace(endpoint.BaseURL)
	if base == "" {
		return nil, errNoBaseURL
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("base URL tidak valid: %s", base)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, fmt.Errorf("skema %q tidak didukung", parsed.Scheme)
	}
	if err := refuseInternal(parsed.Hostname()); err != nil {
		return nil, err
	}

	signature, err := sign(endpoint, order)
	if err != nil {
		return nil, err
	}
	substitutions := map[string]string{
		"sku":         order.SKU,
		"reference":   order.Reference,
		"amount":      strconv.FormatInt(order.AmountIDR, 10),
		"destination": order.Destination,
		"username":    endpoint.Username,
		"credential":  endpoint.Credential,
		"signature":   signature,
		"timestamp":   strconv.FormatInt(time.Now().Unix(), 10),
	}
	// The same substitution twice: once for real, once with the secrets
	// replaced. Building the log line from the real one and redacting
	// afterwards would depend on the credential being distinctive enough to
	// find, which it is not.
	redactions := map[string]string{}
	for key, value := range substitutions {
		redactions[key] = value
	}
	redactions["credential"] = "***"
	if endpoint.Username != "" {
		redactions["username"] = "***"
	}
	if signature != "" {
		redactions["signature"] = "***"
	}

	body := substitute(endpoint.Template, substitutions)
	logged := substitute(endpoint.Template, redactions)

	switch endpoint.Protocol {
	case ProtocolHTTPGet:
		target := base
		if query := strings.TrimSpace(body); query != "" {
			separator := "?"
			if strings.Contains(base, "?") {
				separator = "&"
			}
			target = base + separator + query
		}
		request, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		return &Built{Request: request, Loggable: logged, Endpoint: base}, nil

	case ProtocolFormPost:
		request, err := http.NewRequest(http.MethodPost, base, strings.NewReader(body))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return &Built{Request: request, Loggable: logged, Endpoint: base}, nil

	case ProtocolXMLRPC:
		payload := xmlRPCEnvelope(endpoint.RPCMethod, body)
		logPayload := xmlRPCEnvelope(endpoint.RPCMethod, logged)
		request, err := http.NewRequest(http.MethodPost, base, strings.NewReader(payload))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Content-Type", "text/xml")
		return &Built{Request: request, Loggable: logPayload, Endpoint: base}, nil

	default: // ProtocolRESTJSON
		method := endpoint.Method
		if method == "" {
			method = http.MethodPost
		}
		request, err := http.NewRequest(method, base, strings.NewReader(body))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Content-Type", "application/json")
		return &Built{Request: request, Loggable: logged, Endpoint: base}, nil
	}
}

// substitute replaces {{name}} placeholders. Unknown placeholders are left
// alone rather than blanked: a template referring to something this code does
// not provide is a configuration mistake, and leaving it visible in the log is
// how somebody notices.
func substitute(template string, values map[string]string) string {
	result := template
	for key, value := range values {
		result = strings.ReplaceAll(result, "{{"+key+"}}", value)
	}
	return result
}

// sign builds the hash these terminals expect, from a recipe like
// "md5:{{username}}{{credential}}{{reference}}".
//
// md5 and sha1 are both weak, and both are what a good number of these
// providers mandate. They are here because refusing them means refusing the
// supplier, not because either is a sound choice in 2026.
func sign(endpoint Endpoint, order Order) (string, error) {
	recipe := strings.TrimSpace(endpoint.SignatureRecipe)
	if recipe == "" {
		return "", nil
	}
	algorithm, template, found := strings.Cut(recipe, ":")
	if !found {
		return "", errBadSignature
	}
	payload := substitute(template, map[string]string{
		"sku":         order.SKU,
		"reference":   order.Reference,
		"amount":      strconv.FormatInt(order.AmountIDR, 10),
		"destination": order.Destination,
		"username":    endpoint.Username,
		"credential":  endpoint.Credential,
	})
	switch strings.ToLower(strings.TrimSpace(algorithm)) {
	case "md5":
		sum := md5.Sum([]byte(payload))
		return hex.EncodeToString(sum[:]), nil
	case "sha1":
		sum := sha1.Sum([]byte(payload))
		return hex.EncodeToString(sum[:]), nil
	case "sha256":
		sum := sha256.Sum256([]byte(payload))
		return hex.EncodeToString(sum[:]), nil
	default:
		return "", fmt.Errorf("%w: %s", errBadSignature, algorithm)
	}
}

func xmlRPCEnvelope(method, params string) string {
	return `<?xml version="1.0"?><methodCall><methodName>` + method +
		`</methodName><params>` + params + `</params></methodCall>`
}

// refuseInternal rejects hosts that resolve into the machine or its network.
//
// Supplier addresses are configuration typed in by a person. Without this, one
// mistyped or malicious entry turns the fulfilment worker into a request
// forwarder aimed at anything the server can reach — cloud metadata endpoints
// being the reason this matters rather than a theoretical concern.
func refuseInternal(host string) error {
	if host == "" {
		return errPrivateAddress
	}
	if strings.EqualFold(host, "localhost") && !testing.Testing() {
		return fmt.Errorf("%w: %s", errPrivateAddress, host)
	}
	addresses, err := net.LookupIP(host)
	if err != nil {
		// Unresolvable is not the same as internal. Let the request fail on its
		// own terms rather than reporting a security refusal for a typo.
		return nil
	}
	for _, address := range addresses {
		// Loopback is permitted in a test binary and nowhere else. Every stub
		// server binds to 127.0.0.1, so without this the outbound path could
		// not be exercised at all — and testing.Testing() cannot be true in a
		// compiled server, so this is not a switch anybody can flip in
		// production or a variable anybody can set by accident.
		//
		// Private and link-local ranges stay refused even under test: those are
		// the addresses that matter (cloud metadata above all), and a test that
		// could reach them would stop proving anything.
		if address.IsLoopback() && testing.Testing() {
			continue
		}
		if address.IsLoopback() || address.IsPrivate() || address.IsLinkLocalUnicast() ||
			address.IsUnspecified() || address.IsLinkLocalMulticast() {
			return fmt.Errorf("%w: %s", errPrivateAddress, address)
		}
	}
	return nil
}
