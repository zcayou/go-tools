package wire

// ID is a value another system assigns, with a few well-known constants.
type ID string

const (
	IDNone ID = ""
	IDRoot ID = "root"
)

// Code is an integer another system assigns.
type Code int

const (
	CodeOK Code = iota
	CodeRetry
)
