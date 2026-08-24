package domain

import "time"

type StorefrontSnapshot struct {
	Draft             []byte
	Published         []byte
	DraftRevision     int64
	PublishedRevision int64
	PublishedAt       *time.Time
}
