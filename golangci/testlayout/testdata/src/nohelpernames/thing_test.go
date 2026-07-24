// blackbox.helper-patterns is explicitly empty, so there is no name this file
// could have taken instead and the setting is what the diagnostic names.
package nohelpernames_test // want `spec-less-test-file: no specs or test functions; blackbox\.helper-patterns is empty, so every file of this kind must carry specs`

import "nohelpernames"

func thingValue() int { return nohelpernames.Thing() }
