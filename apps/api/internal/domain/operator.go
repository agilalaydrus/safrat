package domain

import "time"

type Operator struct {
	ID                string
	BetterAuthOrgID   string
	Name              string
	Country           string
	Email             string
	LicenseNumber     string
	Slug              string
	CreatedAt         time.Time
	LogoURL           string
	Description       string
	WhatsappNumber    string
	Website           string
	Address           string
	City              string
	IsProfileComplete bool
}

// PublicSeasonSummary is the non-sensitive view of a season shown on an
// operator's public profile page (/p/{slug}).
type PublicSeasonSummary struct {
	ID           string
	Name         string
	Type         SeasonType
	StartDate    time.Time
	EndDate      time.Time
	PilgrimCount int32
}

type AuditLog struct {
	ID          string
	Action      string
	EntityType  string
	EntityID    string
	Description string
	CreatedAt   time.Time
	ActorName   string
}
