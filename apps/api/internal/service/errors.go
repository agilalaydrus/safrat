package service

import (
	"errors"
	"fmt"
	"log/slog"

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
	case errors.Is(err, apperror.ErrDailyLimitExceeded), errors.Is(err, apperror.ErrCheckoutAttemptLimit):
		// ResourceExhausted rather than FailedPrecondition: nothing about the
		// request is wrong and retrying it tomorrow will work. The distinction
		// matters to callers deciding whether to offer a retry.
		return connect.NewError(connect.CodeResourceExhausted, err)
	case errors.Is(err, apperror.ErrFailedPrecondition), errors.Is(err, apperror.ErrCheckoutHeldBlocked):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, apperror.ErrConflict):
		return connect.NewError(connect.CodeAborted, err)
	case errors.Is(err, apperror.ErrForbidden):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, apperror.ErrUnauthorized):
		return connect.NewError(connect.CodeUnauthenticated, err)
	default:
		// Unmapped errors are bugs, not expected failure modes — report them.
		// sentry.Init (main.go) is a no-op when SENTRY_DSN is unset, so without
		// this slog line a bug in local dev (no DSN configured) would vanish
		// the moment the client got its generic "internal error" — nothing on
		// the machine that just failed would say why (F2, TUGAS-PANEL-SAAS.md).
		slog.Error("unmapped service error", "method", method, "error", err)
		sentry.CaptureException(fmt.Errorf("%s: %w", method, err))
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}
