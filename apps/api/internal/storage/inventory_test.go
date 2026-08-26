package storage

import "testing"

func TestParseStorefrontObjectKey(t *testing.T) {
	const operatorID = "0f8fad5b-d9cb-469f-a165-70867728950e"

	valid := []struct {
		name, key, kind string
	}{
		{"image", "storefront/" + operatorID + "/hero/photo.webp", "hero"},
		{"article image", "storefront/" + operatorID + "/article/cover.webp", "article"},
		{"audio", "storefront/" + operatorID + "/background-music/track.mp3", "background-music"},
	}
	for _, testCase := range valid {
		t.Run(testCase.name, func(t *testing.T) {
			gotOperator, gotKind, err := ParseStorefrontObjectKey(testCase.key)
			if err != nil {
				t.Fatalf("ParseStorefrontObjectKey(%q): %v", testCase.key, err)
			}
			if gotOperator != operatorID || gotKind != testCase.kind {
				t.Fatalf("parsed operator=%q kind=%q, want %q/%q", gotOperator, gotKind, operatorID, testCase.kind)
			}
		})
	}

	invalid := map[string]string{
		"pending prefix":     "storefront-pending/" + operatorID + "/hero/photo.webp",
		"other prefix":       "backups/" + operatorID + "/hero/photo.webp",
		"nested path":        "storefront/" + operatorID + "/hero/nested/photo.webp",
		"missing name":       "storefront/" + operatorID + "/hero/",
		"folder marker":      "storefront/" + operatorID + "/hero",
		"non-uuid operator":  "storefront/not-a-uuid/hero/photo.webp",
		"unknown kind":       "storefront/" + operatorID + "/invoices/photo.webp",
		"wrong extension":    "storefront/" + operatorID + "/hero/photo.png",
		"audio ext on image": "storefront/" + operatorID + "/hero/photo.mp3",
		"image ext on audio": "storefront/" + operatorID + "/background-music/track.webp",
	}
	for name, key := range invalid {
		t.Run(name, func(t *testing.T) {
			if _, _, err := ParseStorefrontObjectKey(key); err == nil {
				t.Fatalf("ParseStorefrontObjectKey(%q) accepted an invalid key", key)
			}
		})
	}
}

func TestStorefrontObjectReservationKeyMatchesUploadPath(t *testing.T) {
	const operatorID = "0f8fad5b-d9cb-469f-a165-70867728950e"
	object := StorefrontObject{ObjectKey: "storefront/" + operatorID + "/hero/photo.webp"}
	want := "storefront-pending/" + operatorID + "/hero/photo.webp"
	if got := object.ReservationKey(); got != want {
		t.Fatalf("ReservationKey() = %q, want %q", got, want)
	}
}
