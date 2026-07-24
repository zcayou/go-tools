package unchecked

// The suite runs this package
// with every rule turned off.
//
// Everything here would be
// reported with them on, this
// paragraph included, so an
// empty want list is what pins
// the settings as the thing
// that decides.

type kind int

func demand() kind { return kind(0) }

func several() { demand(); demand() }

func wrapped(
	count int,
) kind {
	return kind(count)
}
