package user

import "example.com/memberbound/kinds"

func bounded(k kinds.Kind) bool { return k <= kinds.KindC }
