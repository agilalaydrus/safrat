package domain

// MyAccess is the resolved answer to "who is this Better Auth identity,
// across every role system in this app" — operator staff, group leader,
// and/or linked pilgrim are not mutually exclusive, though in practice
// most identities have exactly one.
type MyAccess struct {
	IsOrgMember   bool
	OrgRole       string
	OperatorID    string
	OperatorName  string
	LeaderGroups  []LeaderGroupSummary
	LinkedPilgrim *PilgrimSummary
	LinkedAgent   *Agent
}

type LeaderGroupSummary struct {
	ID   string
	Name string
}

type PilgrimSummary struct {
	ID            string
	AppAccessCode string
	FullName      string
}
