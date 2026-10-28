package chat

import "example.com/bridges/catalog"

// presence is a private copy of catalog.Presence, mapped by a switch returning
// its members.
type presence int

const (
	presenceOnline presence = iota + 1
	presenceAway
	presenceBusy
	presenceHidden
	presenceOffline
)

func exported(p presence) (catalog.Presence, bool) {
	switch p {
	case presenceOnline:
		return catalog.Online, true
	case presenceAway:
		return catalog.Away, true
	case presenceBusy:
		return catalog.Busy, true
	case presenceHidden:
		return catalog.Hidden, true
	case presenceOffline:
		return catalog.Offline, true
	default:
		return 0, false
	}
}

// status is mapped by a switch assigning catalog's members.
type status int

const (
	statusOnline status = iota + 1
	statusAway
	statusBusy
	statusHidden
	statusOffline
)

func assigned(s status) catalog.Presence {
	var p catalog.Presence
	switch s {
	case statusOnline:
		p = catalog.Online
	case statusAway:
		p = catalog.Away
	case statusBusy:
		p = catalog.Busy
	case statusHidden:
		p = catalog.Hidden
	case statusOffline:
		p = catalog.Offline
	}
	return p
}

// mood is mapped by a map literal.
type mood int

const (
	moodOnline mood = iota + 1
	moodAway
	moodBusy
	moodHidden
	moodOffline
)

var moods = map[mood]catalog.Presence{
	moodOnline:  catalog.Online,
	moodAway:    catalog.Away,
	moodBusy:    catalog.Busy,
	moodHidden:  catalog.Hidden,
	moodOffline: catalog.Offline,
}

// state is mapped from catalog's members, by a switch over the visible enum.
type state int

const (
	stateOnline state = iota + 1
	stateAway
	stateBusy
	stateHidden
	stateOffline
)

func imported(p catalog.Presence) state {
	switch p {
	case catalog.Online:
		return stateOnline
	case catalog.Away:
		return stateAway
	case catalog.Busy:
		return stateBusy
	case catalog.Hidden:
		return stateHidden
	case catalog.Offline:
		return stateOffline
	}
	return 0
}

// listed is a copy a related-enums entry may excuse.
type listed int

const (
	listedOnline listed = iota + 1
	listedAway
	listedBusy
	listedHidden
	listedOffline
)

var listings = map[listed]catalog.Presence{
	listedOnline:  catalog.Online,
	listedAway:    catalog.Away,
	listedBusy:    catalog.Busy,
	listedHidden:  catalog.Hidden,
	listedOffline: catalog.Offline,
}
