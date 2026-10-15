package fixturehelpers_test // want `single-consumer-helper-file: everything relay_helpers_test\.go declares serves only beta_test\.go; move it there`

func viaRelay() int { return base() - 1 }
