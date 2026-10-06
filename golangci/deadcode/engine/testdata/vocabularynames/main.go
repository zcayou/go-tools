// Command vocabularynames compares vocabulary members and renders none.
package main

// Phase is a closed vocabulary: its package declares its members. Its String
// is the published name, and rendering a member goes through phaseNames.
type Phase uint8

const (
	Start Phase = iota + 1
	Stop
)

func (p Phase) String() string { return phaseNames()[p] }

func phaseNames() []string { return []string{"", "start", "stop"} }

// Terminal is another method of the vocabulary, which nothing calls.
func (p Phase) Terminal() bool { return p == Stop }

// Label has a String and no declared member, so it is no vocabulary.
type Label string

func (l Label) String() string { return string(l) }

// Level declares members, but its String takes a pointer receiver, which
// renders no member a constant can name.
type Level uint8

const High Level = 1

func (l *Level) String() string { return "level" }

func main() {
	var label Label = "x"
	var level Level
	_, _, _ = Start == Stop, label, level == High
}
