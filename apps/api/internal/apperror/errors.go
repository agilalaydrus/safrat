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
	// ErrCheckoutAttemptLimit is a rolling gateway-attempt limit. It is kept
	// separate from the rupiah limit because the retry time and user message are
	// different, even though both map to ResourceExhausted at the API boundary.
	ErrCheckoutAttemptLimit = errors.New("checkout attempt limit exceeded")
	// ErrCheckoutHeldBlocked means this buyer already has money in the HELD
	// workflow. A new invoice is refused until staff resolve that transaction.
	ErrCheckoutHeldBlocked = errors.New("unresolved held payment blocks checkout")
)
