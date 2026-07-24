package badnames_test // want `spec-less-test-file: no specs or test functions; a file carrying only helpers must be named helpers_test\.go, fakes_test\.go`

import "badnames"

func widgetValue() int { return badnames.Widget() }
