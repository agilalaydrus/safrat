package apperror

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrAlreadyExists      = errors.New("already exists")
	ErrFailedPrecondition = errors.New("failed precondition")
	ErrConflict           = errors.New("conflict")
	ErrForbidden          = errors.New("forbidden")
	ErrValidation         = errors.New("validation failed")
	ErrUnauthorized       = errors.New("authentication required")

	// ErrDailyLimitExceeded is its own error rather than a generic failed
	// precondition because the caller has to say which limit and how much is
	// left. "Transaksi ditolak" without a number sends a jamaah to customer
	// service to ask a question the system already knows the answer to.
	ErrDailyLimitExceeded = errors.New("daily digital transaction limit exceeded")
)
