package pinning

type Platform struct{}

func (Platform) Error() string { return "platform" }
