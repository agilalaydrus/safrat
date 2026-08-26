package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// StorefrontObject is one live object found in the bucket, already resolved to
// the operator and asset kind encoded in its key.
type StorefrontObject struct {
	ObjectKey  string
	OperatorID string
	Kind       string
	SizeBytes  int64
	PublicURL  string
}

// ReservationKey is the pending key this object would have been uploaded
// through. Using it as the registry's reservation key keeps a backfilled row
// identical to one the normal upload path would have written, so a later
// confirmation of the same object updates that row instead of duplicating it.
func (o StorefrontObject) ReservationKey() string {
	return strings.Replace(o.ObjectKey, "storefront/", "storefront-pending/", 1)
}

// ListStorefrontObjects pages through the live `storefront/` prefix. It never
// mutates the bucket. Keys that do not parse as a storefront asset are returned
// in skipped so the caller can report them rather than silently ignoring them.
func (s *Store) ListStorefrontObjects(ctx context.Context, continuationToken string, limit int32) (objects []StorefrontObject, skipped []string, nextToken string, err error) {
	if s == nil {
		return nil, nil, "", ErrNotConfigured
	}
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucket),
		Prefix:  aws.String("storefront/"),
		MaxKeys: aws.Int32(limit),
	}
	if continuationToken != "" {
		input.ContinuationToken = aws.String(continuationToken)
	}
	page, err := s.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, nil, "", fmt.Errorf("list storefront objects: %w", err)
	}
	for _, item := range page.Contents {
		if item.Key == nil || item.Size == nil {
			continue
		}
		operatorID, kind, parseErr := ParseStorefrontObjectKey(*item.Key)
		if parseErr != nil {
			skipped = append(skipped, *item.Key)
			continue
		}
		objects = append(objects, StorefrontObject{
			ObjectKey:  *item.Key,
			OperatorID: operatorID,
			Kind:       kind,
			SizeBytes:  *item.Size,
			PublicURL:  s.publicBaseURL + "/" + *item.Key,
		})
	}
	if page.NextContinuationToken != nil {
		nextToken = *page.NextContinuationToken
	}
	return objects, skipped, nextToken, nil
}

// ParseStorefrontObjectKey validates a live object key and returns the operator
// and asset kind it encodes. It applies exactly the constraints the upload path
// produces: `storefront/<operator-uuid>/<kind>/<name><ext>`.
func ParseStorefrontObjectKey(objectKey string) (operatorID, kind string, err error) {
	invalid := fmt.Errorf("invalid storefront object key")
	parts := strings.Split(objectKey, "/")
	if len(parts) != 4 || parts[0] != "storefront" {
		return "", "", invalid
	}
	if _, err := uuid.Parse(parts[1]); err != nil {
		return "", "", invalid
	}
	spec, err := storefrontSpec(parts[2])
	if err != nil || parts[3] == "" || !strings.HasSuffix(parts[3], spec.extension) {
		return "", "", invalid
	}
	return parts[1], parts[2], nil
}
