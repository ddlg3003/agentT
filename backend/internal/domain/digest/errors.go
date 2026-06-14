package digest

import "errors"

// ErrNotFound is returned by Repository.Get when no digest exists for a date.
var ErrNotFound = errors.New("digest not found")
