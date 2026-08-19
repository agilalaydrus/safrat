package domain

import "time"

type ChecklistTemplate struct {
	ID          string
	OperatorID  string
	SeasonID    string
	Title       string
	Description string
	Category    string
	IsRequired  bool
	SortOrder   int32
}

type ChecklistItem struct {
	TemplateID  string
	Title       string
	Description string
	Category    string
	IsRequired  bool
	IsCompleted bool
	CompletedBy string
	Notes       string
	CompletedAt *time.Time
}

type ChecklistStat struct {
	TemplateID     string
	Title          string
	Category       string
	IsRequired     bool
	CompletedCount int64
	TotalPilgrims  int64
}
