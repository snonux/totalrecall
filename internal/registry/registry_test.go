package registry

import "testing"

func TestRegistryRegisterGet(t *testing.T) {
	r := New[string, int]()
	r.Register("a", 1)
	r.Register("b", 2)

	v, ok := r.Get("a")
	if !ok || v != 1 {
		t.Fatalf("Get(a) = %v, %v, want 1, true", v, ok)
	}
	_, ok = r.Get("missing")
	if ok {
		t.Fatal("Get(missing) should be false")
	}
}

func TestRegistryDuplicatePanics(t *testing.T) {
	r := New[string, int]()
	r.Register("a", 1)
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate Register")
		}
	}()
	r.Register("a", 2)
}
