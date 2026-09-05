package domain

import "time"

// StorefrontInquiry is a visitor's "Hubungi Kami" submission from a tenant
// storefront. It carries whatever utm_source/utm_campaign the visitor's link
// had, captured at submission time — the whole reason converting one into a
// crm_lead can fill Source/Campaign truthfully instead of a staff member
// typing a guess.
type StorefrontInquiry struct {
	ID              string
	OperatorID      string
	FullName        string
	Phone           string
	Email           string
	Message         string
	UTMSource       string
	UTMCampaign     string
	Status          string // NEW | CONVERTED | DISMISSED
	ConvertedLeadID string
	CreatedAt       time.Time
}
