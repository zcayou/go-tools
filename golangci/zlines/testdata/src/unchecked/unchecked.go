package unchecked

// The suite runs this package with signature-wrap and body-collapse disabled.
// Both would report what follows, so an empty want list pins the selection
// as the thing that decides.

type kind int

func demand() kind { return kind(0) }

func several() { demand(); demand() }

func wrapped(
	count int,
) kind {
	return kind(count)
}
