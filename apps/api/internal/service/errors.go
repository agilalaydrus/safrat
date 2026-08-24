package service

import (
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/getsentry/sentry-go"
	"github.com/hajj-saas/api/internal/apperror"
)

// serviceError ensures services return protocol-safe errors while retaining causes for logs.
func serviceError(method string, err error) error {
	if err == nil {
		return nil
	}

	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		return err
	}

	switch {
	case errors.Is(err, apperror.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, apperror.ErrAlreadyExists):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, apperror.ErrValidation):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, apperror.ErrFailedPrecondition):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, apperror.ErrConflict):
		return connect.NewError(connect.CodeAborted, err)
	case errors.Is(err, apperror.ErrForbidden):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, apperror.ErrUnauthorized):
		return connect.NewError(connect.CodeUnauthenticated, err)
	default:
		// Unmapped errors are bugs, not expected failure modes — report them.
		// sentry.Init (main.go) is a no-op when SENTRY_DSN is unset, so this is
		// safe to call unconditionally in dev.
		sentry.CaptureException(fmt.Errorf("%s: %w", method, err))
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}
