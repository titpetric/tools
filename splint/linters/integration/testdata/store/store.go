// Package store opens the real storage.
package store

// Store is the storage.
type Store struct{ DSN string }

// New opens a store.
func New(dsn string) *Store { return &Store{DSN: dsn} }
