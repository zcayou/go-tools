// reader_test.go names shadowed in every way that declares something else
// by the name, and refers to the keys only as map keys.
package fixturescopes_test

import "testing"

func TestScopes(t *testing.T) {
	if shadowed != 1 {
		t.Fatal("scopes")
	}
}

var shadowed = 1

var mapKey = 1 // want `misplaced-fixture: mapKey is used only by reader_test\.go; declare it there`

var elidedKey = 2 // want `misplaced-fixture: elidedKey is used only by reader_test\.go; declare it there`

var namedKey = 3 // want `misplaced-fixture: namedKey is used only by reader_test\.go; declare it there`
