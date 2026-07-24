package narrow

// The suite runs this package at a limit of 34, the exact width of the line
// atTheLimit joins to, which is what pins the limit as a width the joined line
// is allowed to reach rather than one it has to stay under.

// want +1 `signature-wrap: atTheLimit: signature is wrapped but fits on one 34-column line \(limit 34\)`
func atTheLimit(
	count int,
) error {
	return nil
}

func pastTheLimit(
	count int,
	name string,
) error {
	return nil
}

// A collapsed body is not part of the line the fix would leave, so it is not
// part of the width either: counted in, this signature would sit past the limit
// on the first run and be reported only on the second, once the body-collapse
// fix had taken the body off the line.

// want +2 `signature-wrap: folded: signature is wrapped but fits on one 28-column line \(limit 34\)`
// want +3 `body-collapse: folded: body is written on the signature line rather than lines of its own`
func folded(
	count int,
) int { return count * 2 }
