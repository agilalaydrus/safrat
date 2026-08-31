package domain

type Entitlement struct {
	MaxPilgrims  *int32
	MaxBranches  *int32
	Features     map[string]bool
	PilgrimCount int32
	BranchCount  int32
}
