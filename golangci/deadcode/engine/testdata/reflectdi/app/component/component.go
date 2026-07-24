// Package component holds a plain concrete component bound the same
// reflect-typed way as the generic facade.
package component

// Store is the capability main discovers on the cache.
type Store interface {
	Save(state string)
}

// Cache reaches the derived-type closure through its registered constructor's
// signature.
type Cache struct {
	saved []string
}

func NewCache() *Cache { return &Cache{} }

func (c *Cache) Save(state string) {
	c.saved = append(c.saved, state)
}
