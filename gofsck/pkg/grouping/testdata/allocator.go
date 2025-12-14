package model

type Allocator[T any] struct {
	store map[string]any
}

func (a *Allocator[T]) Put(k string, val T) {
	a.store[k] = val
}
