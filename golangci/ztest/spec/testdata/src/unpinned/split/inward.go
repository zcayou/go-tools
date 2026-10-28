package split

type Inward struct{} // want Inward:"rendering"

func (Inward) Error() string { return "inward" }
