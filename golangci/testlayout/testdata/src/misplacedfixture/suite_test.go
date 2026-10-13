package misplacedfixture_test

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	setUp()
	os.Exit(m.Run())
}
