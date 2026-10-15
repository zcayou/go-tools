package fixtureboundary

import "testing"

func TestInternal(t *testing.T) {
	if thing() != 1 {
		t.Fatal("internal")
	}
}

func Built() int { return 2 } // want `misplaced-fixture: Built is used by thing_test\.go; a fixture another file uses belongs in a file named export_test\.go`

func Dotted() int { return 3 } // want `misplaced-fixture: Dotted is used by other_test\.go; a fixture another file uses belongs in a file named export_test\.go`
