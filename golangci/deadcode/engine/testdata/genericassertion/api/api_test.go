package api_test

import (
	"testing"

	"genericassertion/api"
)

// getter is the test's own interface over Handle[int], the way a consumer would
// declare one.
type getter interface{ Get() int }

func TestBind(t *testing.T) {
	s := api.NewStore()
	api.Put(s, "x", 1)
	g, ok := api.Bind[getter](s, "x")
	if !ok || g.Get() != 1 || api.Total(s) != 3 {
		t.Fatal("bind")
	}
}
