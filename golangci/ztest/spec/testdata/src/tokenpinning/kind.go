package tokenpinning

type Kind int // want Kind:"enum"

const (
	KindA Kind = iota + 1
	KindB
)
