package a

import "example.com/foreign"

// Day keeps the foreign module's enum in use.
var Day = foreign.Monday

type Color int

const (
	Red Color = iota + 1
	Green
)

func (c Color) String() string {
	switch c {
	case Red:
		return "red"
	case Green:
		return "green"
	default:
		return "unknown"
	}
}

type Shade int

const (
	Dark Shade = iota + 1
	Light
	Dim
)

func (s Shade) String() string {
	switch s {
	case Dark:
		return "dark"
	case Light:
		return "light"
	case Dim:
		return "dim"
	default:
		return "unknown"
	}
}

type Size int

const (
	Small Size = iota + 1
	Large
)

func (s Size) String() string {
	switch s {
	case Small:
		return "small"
	case Large:
		return "large"
	default:
		return "unknown"
	}
}

type Solo int

const Only Solo = 1

func (Solo) String() string { return "only" }

type Status int

const (
	Open Status = iota + 1
	Closed
)

func (s Status) String() string {
	switch s {
	case Open:
		return "open"
	case Closed:
		return "closed"
	default:
		return "unknown"
	}
}
