// Each fixture here is judged by the files whose specs reach it, not by the file
// declaring it.
package misplacedfixture_test

import "testing"

func TestStore(t *testing.T) {
	if newStore()+storeOnly()+private != 3 {
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

func inner() int { return 1 } // want `misplaced-fixture: inner is used only by ledger_test\.go; declare it there`

// setUp serves the suite file, which may hold nothing but its entry point.
func setUp() {} // want `misplaced-fixture: setUp is used by suite_test\.go; a fixture another file uses belongs in a file named helpers_test\.go, fakes_test\.go`

func unused() int { return 0 }

// stamp is reached only through a method of a type nothing uses.
func stamp() int { return 0 }
