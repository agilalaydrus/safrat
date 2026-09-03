package funnel

import "context"

// Recorder records one funnel step for a request that is doing something else.
//
// An interface rather than the service itself so the handlers that use it do
// not gain a dependency on the funnel package's internals, and so a handler can
// be built without one at all — recording is optional everywhere by design.
type Recorder interface {
	Record(ctx context.Context, step Step)
}

// Step is everything needed to attribute one event, gathered by the caller
// because only it can see the request.
type Step struct {
	OperatorID  string
	Step        string
	Path        string
	UTMSource   string
	UTMCampaign string
	EntityID    string
	ClientIP    string
	UserAgent   string
}
