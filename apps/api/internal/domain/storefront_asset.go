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
