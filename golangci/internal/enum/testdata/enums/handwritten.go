package enums

// Chain names its members in an if chain, and leaves ChainC to the return
// after it.
type Chain int

const (
	ChainA Chain = iota + 1
	ChainB
	ChainC
)

func (c Chain) String() string {
	if c == ChainA {
		return "chain-a"
	} else if c == ChainB {
		return "chain-b"
	}
	return "unknown"
}

// Tagless names its members in a switch with no tag.
type Tagless int

const (
	TaglessA Tagless = iota + 1
	TaglessB
)

func (t Tagless) String() string {
	switch {
	case t == TaglessA:
		return "tagless-a"
	case t == TaglessB:
		return "tagless-b"
	}
	return "unknown"
}

// Through falls through from one case into the next, and leaves ThroughC
// to the default arm.
type Through int

const (
	ThroughA Through = iota + 1
	ThroughB
	ThroughC
)

func (t Through) String() string {
	switch t {
	case ThroughA:
		fallthrough
	case ThroughB:
		return "through"
	default:
		return "unknown"
	}
}

// Keyed indexes an array keyed by member, behind a bounds check.
type Keyed int

const (
	KeyedA Keyed = iota + 1
	KeyedB
)

var keyedNames = [...]string{KeyedA: "keyed-a", KeyedB: "keyed-b"}

func (k Keyed) String() string {
	if int(k) < len(keyedNames) {
		return keyedNames[k]
	}
	return "unknown"
}

// CommaOk looks its names up by hand, and lacks one for CommaOkB.
type CommaOk int

const (
	CommaOkA CommaOk = iota + 1
	CommaOkB
)

var commaOkNames = map[CommaOk]string{CommaOkA: "comma-ok-a"}

func (c CommaOk) String() string {
	name, ok := commaOkNames[c]
	if !ok {
		return "unknown"
	}
	return name
}

// Holey indexes a map that lacks HoleyB, with no check.
type Holey int

const (
	HoleyA Holey = iota + 1
	HoleyB
)

var holeyNames = map[Holey]string{HoleyA: "holey-a"}

func (h Holey) String() string {
	return holeyNames[h]
}

// Panicky panics for PanickyB, which its switch has no case for.
type Panicky int

const (
	PanickyA Panicky = iota + 1
	PanickyB
)

func (p Panicky) String() string {
	switch p {
	case PanickyA:
		return "panicky-a"
	default:
		panic("no name")
	}
}

// Local builds its names in locals: a constant, a variable assigned in each
// arm, and a counter stepped down.
type Local int

const (
	LocalA Local = iota + 1
	LocalB
)

func (l Local) String() string {
	const prefix = "local-"
	var name string
	i := int(l)
	i--
	switch i {
	case 0:
		name = prefix + "a"
	case 1:
		name = prefix + "b"
	default:
		name = "unknown"
	}
	return name
}

// Wrapped reads its names at an offset its unsigned values wrap past: 0 less
// 254 is 2 in a byte.
type Wrapped uint8

const (
	WrappedHigh Wrapped = 254
	WrappedTop  Wrapped = 255
	WrappedZero Wrapped = 0
)

var wrappedNames = [...]string{"high", "top", "zero"}

func (w Wrapped) String() string {
	w -= 254
	if w >= Wrapped(len(wrappedNames)) {
		return "unknown"
	}
	return wrappedNames[w]
}

// MapLen checks its index against how many names its map holds, which
// leaves MapLenTwo out.
type MapLen int

const (
	MapLenOne MapLen = iota + 1
	MapLenTwo
)

var mapLenNames = map[MapLen]string{MapLenOne: "map-len-one", MapLenTwo: "map-len-two"}

func (m MapLen) String() string {
	if int(m) >= len(mapLenNames) {
		return "unknown"
	}
	return mapLenNames[m]
}

// Special names SpecialB in the same condition that checks the table's bounds.
type Special int

const (
	SpecialA Special = iota + 1
	SpecialB
)

var specialNames = [...]string{"", "special-a"}

func (s Special) String() string {
	if s == SpecialB || int(s) >= len(specialNames) {
		return "special-b"
	}
	return specialNames[s]
}

// Constant renders one string for every member, which names none of them.
type Constant int

const (
	ConstantA Constant = iota + 1
	ConstantB
)

func (Constant) String() string { return "constant" }

// Delegating computes its names in another method.
type Delegating int

const DelegatingA Delegating = 1

func (d Delegating) String() string {
	if d == DelegatingA {
		return d.name()
	}
	return "unknown"
}

func (Delegating) name() string { return "delegating-a" }

// Unnamed names no member: every one reaches the return after its check.
type Unnamed int

const UnnamedA Unnamed = 1

func (u Unnamed) String() string {
	if u > 100 {
		return "large"
	}
	return "unnamed"
}
