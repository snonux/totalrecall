// Package registry provides a small generic keyed map for wiring named factories
// without central type switches (Open/Closed). K is usually string; T is the
// constructor or builder function type for that key.
package registry

// Registry maps comparable keys to values (typically factory functions).
// It is intentionally minimal: no mutex; callers register at init/package load.
type Registry[K comparable, T any] struct {
	m map[K]T
}

// New returns an empty Registry. Call Register for each supported key.
func New[K comparable, T any]() *Registry[K, T] {
	return &Registry[K, T]{m: make(map[K]T)}
}

// Register associates key with value. It panics if key is already registered
// so duplicate wiring is caught at startup.
func (r *Registry[K, T]) Register(key K, value T) {
	if r == nil {
		panic("registry: Register on nil Registry")
	}
	if r.m == nil {
		r.m = make(map[K]T)
	}
	if _, exists := r.m[key]; exists {
		panic("registry: duplicate registration for key")
	}
	r.m[key] = value
}

// Get returns the value for key and whether it was found.
func (r *Registry[K, T]) Get(key K) (T, bool) {
	var zero T
	if r == nil || r.m == nil {
		return zero, false
	}
	v, ok := r.m[key]
	return v, ok
}
