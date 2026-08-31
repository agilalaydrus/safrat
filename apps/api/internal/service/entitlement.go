package service

import (
	"context"
	"fmt"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/repository"
)

type EntitlementChecker struct {
	repository *repository.EntitlementRepository
}

func NewEntitlementChecker(repository *repository.EntitlementRepository) *EntitlementChecker {
	return &EntitlementChecker{repository: repository}
}

// Check is the service-facing policy gate. Database triggers remain the
// concurrency-safe authority; this returns a useful precondition before the
// write is attempted in ordinary requests.
func (c *EntitlementChecker) Check(ctx context.Context, operatorID, resource string) error {
	if c == nil || c.repository == nil {
		return nil
	}
	entitlement, err := c.repository.Get(ctx, operatorID)
	if err != nil {
		return err
	}
	switch resource {
	case "pilgrims":
		if entitlement.MaxPilgrims != nil && entitlement.PilgrimCount >= *entitlement.MaxPilgrims {
			return fmt.Errorf("%w: batas %d jamaah sudah tercapai", apperror.ErrFailedPrecondition, *entitlement.MaxPilgrims)
		}
	case "branches":
		if !entitlement.Features["branches"] {
			return fmt.Errorf("%w: fitur cabang tidak tersedia di paket ini", apperror.ErrFailedPrecondition)
		}
		if entitlement.MaxBranches != nil && entitlement.BranchCount >= *entitlement.MaxBranches {
			return fmt.Errorf("%w: batas %d cabang sudah tercapai", apperror.ErrFailedPrecondition, *entitlement.MaxBranches)
		}
	default:
		return fmt.Errorf("%w: resource entitlement tidak dikenal", apperror.ErrValidation)
	}
	return nil
}
