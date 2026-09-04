package domain

import "time"

// SecuritySettings is one operator's IP allowlist configuration. Enabled
// defaults to false, and the CHECK constraint behind it (migration 161)
// guarantees Enabled is never true with an empty CIDRs list.
type SecuritySettings struct {
	OperatorID string
	Enabled    bool
	CIDRs      []string
}

// ActiveSession is one live Better Auth session belonging to a member of an
// operator's organization.
type ActiveSession struct {
	ID        string
	UserName  string
	UserEmail string
	IPAddress string
	UserAgent string
	CreatedAt time.Time
}
