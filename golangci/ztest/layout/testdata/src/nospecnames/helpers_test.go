// blackbox.patterns is explicitly empty, so there is no file to send these
// specs to and the setting is what the diagnostic names.
package nospecnames_test // want `specs-in-helper-file: a file named for its supporting role carries specs; blackbox\.patterns is empty, so no file of this kind may carry specs`

import "testing"

func TestHelpers(t *testing.T) { t.Log("a helper file has no business holding this") }
