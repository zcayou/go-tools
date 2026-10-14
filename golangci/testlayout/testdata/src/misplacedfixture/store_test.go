// Each fixture here is judged by the files whose specs reach it, not by the file
// declaring it.
package misplacedfixture_test

import "testing"

func TestStore(t *testing.T) {
	if newStore()+storeOnly()+private+refusing() != 4 {
		t.Fatal("store")
	}
}

func newStore() int { return 1 } // want `misplaced-fixture: newStore is used by cache_test\.go, store_test\.go; a fixture another file uses belongs in a file named helpers_test\.go, fakes_test\.go`

func cacheSize() int { return 2 } // want `misplaced-fixture: cacheSize is used only by cache_test\.go; declare it there`

func storeOnly() int { return 1 }

var (
	shared  = 1 // want `misplaced-fixture: shared is used only by queue_test\.go; declare it there`
	private = 1
)

// inner is reached only through outer, so it belongs wherever outer does.
func outer() int { return inner() } // want `misplaced-fixture: outer is used only by ledger_test\.go; declare it there`

func inner() int { return deepest() } // want `misplaced-fixture: inner is used only by ledger_test\.go \(reached through outer\); declare it there`

func deepest() int { return 1 } // want `misplaced-fixture: deepest is used only by ledger_test\.go \(reached through outer, then inner\); declare it there`

// offerScope is used by cache_test.go itself, and reached by queue_test.go only
// through registryDependency.
func registryDependency() int { return offerScope() } // want `misplaced-fixture: registryDependency is used only by queue_test\.go; declare it there`

func offerScope() int { return 1 } // want `misplaced-fixture: offerScope is used by cache_test\.go, queue_test\.go \(reached through registryDependency\); a fixture another file uses belongs in a file named helpers_test\.go, fakes_test\.go`

// storeFull is named by store_test.go, which declares it and reaches it through
// its own refusing, so only queue_test.go is shown the chain.
func refusing() int { return storeFull() } // want `misplaced-fixture: refusing is used by queue_test\.go, store_test\.go; a fixture another file uses belongs in a file named helpers_test\.go, fakes_test\.go`

func storeFull() int { return 1 } // want `misplaced-fixture: storeFull is used by queue_test\.go \(reached through refusing\), store_test\.go; a fixture another file uses belongs in a file named helpers_test\.go, fakes_test\.go`

// flushed is named by cache_test.go, in the method it declares on writer.
type writer struct{} // want `misplaced-fixture: writer is used only by cache_test\.go; declare it there`

func flushed() int { return 1 } // want `misplaced-fixture: flushed is used only by cache_test\.go; declare it there`

// setUp serves the suite file, which may hold nothing but its entry point.
func setUp() {} // want `misplaced-fixture: setUp is used by suite_test\.go; a fixture another file uses belongs in a file named helpers_test\.go, fakes_test\.go`

func unused() int { return 0 }

// stamp is reached only through a method of a type nothing uses.
func stamp() int { return 0 }
