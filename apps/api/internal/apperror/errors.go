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
)
