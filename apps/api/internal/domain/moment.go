package domain

import "time"

// Moment is a photo and a short note sent to a pilgrim's family. Targets
// exactly one of PilgrimID or GroupID — never both, never neither — so it
// always has a knowable audience.
type Moment struct {
	ID          string
	OperatorID  string
	SeasonID    string
	PilgrimID   string
	PilgrimName string
	GroupID     string
	GroupName   string
	PhotoKey    string
	Caption     string
	CreatedBy   string
	CreatedAt   time.Time
}

// FamilyMoment is the privacy-trimmed shape a family member sees — no
// operator/pilgrim/group id, no raw storage key, just what GetFamilyStatus
// already withholds nothing more than: a picture, a caption, a time.
type FamilyMoment struct {
	PhotoKey  string
	Caption   string
	CreatedAt time.Time
}
