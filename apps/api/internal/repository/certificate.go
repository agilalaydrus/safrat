package repository

import (
	"context"

	"github.com/hajj-saas/api/internal/domain"
)

func (r *PilgrimRepository) GetCertificateData(ctx context.Context, appAccessCode string) (*domain.CertificateData, error) {
	row, err := r.queries.GetCertificateData(ctx, appAccessCode)
	if err != nil {
		return nil, databaseError(err)
	}
	return &domain.CertificateData{
		PilgrimName: row.FullName, PassportNumber: openKYC(row.PassportNumber), Nationality: row.Nationality,
		SeasonName: row.SeasonName, SeasonType: domainSeasonType(row.SeasonType),
		StartDate: row.StartDate.Time, EndDate: row.EndDate.Time,
		OperatorName: row.OperatorName, LicenseNumber: row.LicenseNumber.String,
		GroupName: row.GroupName, LeaderName: row.LeaderName,
		HotelsVisited: stringOrEmpty(row.HotelsVisited), MakkahHotels: stringOrEmpty(row.MakkahHotels), MadinahHotels: stringOrEmpty(row.MadinahHotels),
	}, nil
}

func stringOrEmpty(value interface{}) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}
