package chat

import "example.com/bridges/catalog"

// partial leaves a member unpaired.
type partial int

const (
	partialOnline partial = iota + 1
	partialAway
	partialBusy
	partialHidden
	partialOffline
)

func fromPartial(p partial) catalog.Presence {
	switch p {
	case partialOnline:
		return catalog.Online
	case partialAway:
		return catalog.Away
	case partialBusy:
		return catalog.Busy
	case partialHidden:
		return catalog.Hidden
	}
	return catalog.Offline
}

// merged pairs two of its members with one of catalog's.
type merged int

const (
	mergedOnline merged = iota + 1
	mergedAway
	mergedBusy
	mergedHidden
	mergedOffline
)

func fromMerged(m merged) catalog.Presence {
	switch m {
	case mergedOnline:
		return catalog.Online
	case mergedAway, mergedBusy:
		return catalog.Away
	case mergedHidden:
		return catalog.Hidden
	case mergedOffline:
		return catalog.Offline
	}
	return 0
}

// Wire is a visible enum of its own, bridged to catalog's: a vocabulary
// readers meet, with tokens of its own.
type Wire int

const (
	WireOnline Wire = iota + 1
	WireAway
	WireBusy
	WireHidden
	WireOffline
)

func (w Wire) String() string {
	switch w {
	case WireOnline:
		return "wire-online"
	case WireAway:
		return "wire-away"
	case WireBusy:
		return "wire-busy"
	case WireHidden:
		return "wire-hidden"
	case WireOffline:
		return "wire-offline"
	default:
		return "unknown"
	}
}

var wires = map[Wire]catalog.Presence{
	WireOnline:  catalog.Online,
	WireAway:    catalog.Away,
	WireBusy:    catalog.Busy,
	WireHidden:  catalog.Hidden,
	WireOffline: catalog.Offline,
}
