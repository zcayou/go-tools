// Package facade holds the generic facade whose capabilities are invoked only
// through the Authority interface.
package facade

// Authority is the interface main works the runtime through.
type Authority interface {
	Pull() string
	Push(item string)
}

// Runtime reaches the derived-type closure only through the registered
// constructor's signature: func() *Runtime[C], boxed as any.
type Runtime[C any] struct {
	config C
	items  []string
}

// NewRuntime returns the constructor the container registers.
func NewRuntime[C any](config C) func() *Runtime[C] {
	return func() *Runtime[C] { return &Runtime[C]{config: config} }
}

func (r *Runtime[C]) Pull() string {
	if len(r.items) == 0 {
		return "empty"
	}
	item := r.items[len(r.items)-1]
	r.items = r.items[:len(r.items)-1]
	return item
}

func (r *Runtime[C]) Push(item string) {
	r.items = append(r.items, item)
}

// Orphan is declared by no interface and called by nothing: reflection could
// find it, and that is the whole of its liveness.
func (r *Runtime[C]) Orphan() {
	r.items = nil
}
