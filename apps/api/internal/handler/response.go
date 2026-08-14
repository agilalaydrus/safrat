package handler

import (
	"errors"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/apperror"
)

func connectError(err error) error {
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		return err
	}

	switch {
	case errors.Is(err, apperror.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("record not found"))
	case errors.Is(err, apperror.ErrForbidden):
		return connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	case errors.Is(err, apperror.ErrValidation):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	case errors.Is(err, apperror.ErrUnauthorized):
		return connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	default:
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}
