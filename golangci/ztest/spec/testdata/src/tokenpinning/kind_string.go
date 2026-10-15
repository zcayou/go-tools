package tokenpinning

func (k Kind) String() string {
	switch k {
	case KindA:
		return "a"
	case KindB:
		return "b"
	default:
		return "?"
	}
}
