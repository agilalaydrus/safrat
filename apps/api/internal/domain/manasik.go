package domain

import "time"

type ManasikCurriculum struct {
	ID          string
	OperatorID  string
	SeasonID    string
	Title       string
	Description string
	SortOrder   int32
	CreatedAt   time.Time
}

type ManasikSession struct {
	ID              string
	OperatorID      string
	SeasonID        string
	CurriculumID    string
	CurriculumTitle string
	KloterID        string
	KloterCode      string
	Title           string
	Location        string
	InstructorName  string
	ScheduledAt     time.Time
	DurationMinutes int32
	Capacity        int32
	Notes           string
	Status          string // SCHEDULED | COMPLETED | CANCELLED
	CreatedAt       time.Time
}

type ManasikAttendance struct {
	ID             string
	OperatorID     string
	SessionID      string
	PilgrimID      string
	PilgrimName    string
	PassportNumber string
	Status         string // PRESENT | ABSENT | EXCUSED
	Notes          string
	RecordedAt     time.Time
}

type ManasikAttendanceSummary struct {
	PresentCount int32
	AbsentCount  int32
	ExcusedCount int32
}
