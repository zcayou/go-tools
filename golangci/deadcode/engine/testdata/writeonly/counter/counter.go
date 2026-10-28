// Package counter adds to atomic fields, some for the value Add returns.
package counter

import (
	"fmt"
	"sync/atomic"
)

type numbered struct{ ordinal int32 }

type factory struct{ next atomic.Int32 }

type addedOnly struct{ hits atomic.Int32 }

type blanked struct{ old atomic.Int64 }

type deferred struct{ n int64 }

type swapped struct{ last atomic.Int64 }

type summed struct{ total int64 }

func Use() string {
	f := &factory{}
	a := &addedOnly{}
	a.hits.Add(1)
	b := &blanked{}
	_ = b.old.Swap(1)
	d := &deferred{}
	defer atomic.AddInt64(&d.n, 1)
	s := &swapped{}
	t := &summed{}
	if s.last.Swap(2) > atomic.AddInt64(&t.total, 1) {
		return ""
	}
	return fmt.Sprint(numbered{ordinal: f.next.Add(1)}, numbered{ordinal: f.next.Add(1)})
}
