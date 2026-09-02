package domain

import "time"

type CRMLead struct {
	ID, OperatorID, BranchID                 string
	FullName, Phone, Email, Source, Campaign string
	Stage, SeasonID, SeasonName              string
	ProductID, ProductName                   string
	AssigneeUserID, AssigneeName             string
	Pax                                      int32
	EstimatedValueIDR                        int64
	NextAction                               string
	NextFollowUpAt, LastContactAt, ClosedAt  *time.Time
	CreatedAt, UpdatedAt                     time.Time
}

type CRMLeadDraft struct {
	FullName, Phone, Email, Source, Campaign string
	SeasonID, ProductID, AssigneeUserID      string
	Pax                                      int32
	EstimatedValueIDR                        int64
	NextAction                               string
	NextFollowUpAt                           *time.Time
	Note, IdempotencyKey                     string
}

type CRMLeadFilter struct {
	Stage, Source, Search string
	Limit, Offset         int32
}

type CRMLeadActivity struct {
	ID, LeadID, Kind, FromStage, ToStage string
	Note, ActorUserID, ActorName         string
	OccurredAt                           time.Time
}

type CRMLeadDetail struct {
	Lead       CRMLead
	Activities []CRMLeadActivity
}

type CRMStageMetric struct {
	Stage           string
	Count, ValueIDR int64
}

type CRMSourceMetric struct {
	Source          string
	Count, ValueIDR int64
}

type CRMAssigneeMetric struct {
	UserID, Name                        string
	ActiveCount, ClosingCount, ValueIDR int64
}

type CRMDashboard struct {
	ActiveCount, PipelineValueIDR, OverdueFollowUpCount, SourceCount int64
	MonthlyConversionBPS                                             int32
	Stages                                                           []CRMStageMetric
	Sources                                                          []CRMSourceMetric
	Assignees                                                        []CRMAssigneeMetric
	AttentionLeads                                                   []CRMLead
	UpdatedAt                                                        time.Time
}

type CRMAssignee struct{ UserID, Name, Email string }
