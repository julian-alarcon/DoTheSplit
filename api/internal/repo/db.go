// Package repo holds Postgres repositories and shared DB types.
package repo

import "errors"

// ErrNotFound is returned by repo methods when no row matches.
var ErrNotFound = errors.New("not found")
