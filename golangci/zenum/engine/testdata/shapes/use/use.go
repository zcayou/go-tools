package use

import "example.com/shapes/kinds"

func both(f kinds.Flags) bool { return f&(kinds.FlagA|kinds.FlagB) == kinds.FlagA|kinds.FlagB }

func size(n int) kinds.Bytes { return kinds.Bytes(n) * kinds.KB }

func large(b kinds.Bytes) bool { return b == 4096 }
