package domain

type StorefrontAsset struct {
	ObjectKey  string
	OperatorID string
}

type StorefrontAssetReservation struct {
	ReservationKey string
	OperatorID     string
}

type StorefrontStorageUsage struct {
	UsedBytes    int64
	AssetCount   int32
	PendingCount int32
}

// StorefrontAssetImport is one pre-registry object being adopted into the
// asset registry by the one-time backfill.
type StorefrontAssetImport struct {
	ReservationKey string
	ObjectKey      string
	OperatorID     string
	Kind           string
	PublicURL      string
	SizeBytes      int64
}

// StorefrontBackfillReport accounts for every candidate object in a backfill
// batch, so nothing is skipped without being counted.
type StorefrontBackfillReport struct {
	Inserted          int64
	AlreadyRegistered int64
	UnknownOperator   int64
	InvalidSize       int64
}

// Add accumulates a per-batch report into a running total.
func (r *StorefrontBackfillReport) Add(other StorefrontBackfillReport) {
	r.Inserted += other.Inserted
	r.AlreadyRegistered += other.AlreadyRegistered
	r.UnknownOperator += other.UnknownOperator
	r.InvalidSize += other.InvalidSize
}
