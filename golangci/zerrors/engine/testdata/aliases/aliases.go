// Package aliases exports types declared in an internal package under
// aliases of its own.
package aliases

import "example.com/aliases/internal/store"

// NotFound names the store's error type.
type NotFound = store.NotFound

// Store names the store.
type Store = store.Store

func Find() error { return &store.NotFound{} }

func Hidden() error { return &store.Hidden{} }
