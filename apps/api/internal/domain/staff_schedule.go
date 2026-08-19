package domain

import "time"

type KloterStaff struct {
	ID            string
	OperatorID    string
	KloterID      string
	KloterName    string
	StaffID       string
	StaffName     string
	StaffEmail    string
	Role          string
	Duties        string
	DepartureDate *time.Time
	SeasonName    string
	CreatedAt     time.Time
}

type KloterScheduleSummary struct {
	KloterID      string
	KloterName    string
	SeasonName    string
	StaffCount    int64
	StaffNames    string
	DepartureDate *time.Time
}
