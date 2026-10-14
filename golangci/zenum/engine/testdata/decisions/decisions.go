package decisions

import (
	"fmt"
	"strings"

	"example.com/decisions/kinds"
)

func rendered(k kinds.Kind, m kinds.Mode, l kinds.Labeled) {
	if k.String() == "a" {
	}
	if strings.HasPrefix(k.String(), "a") {
	}
	if strings.ToLower(k.String()) == "a" {
	}
	if string(m) == "read" {
	}
	if fmt.Sprint(k) == "a" {
	}
	if l.String() == "a" {
	}
	switch k.String() {
	case "a":
	case "b":
	}
	if k.String() == "" {
	}
	if k.String() == string(m) {
	}
}

func compared(k kinds.Kind, m kinds.Mode, l kinds.Level) {
	if k == 7 {
	}
	if 7 != k {
	}
	switch k {
	case kinds.KindA:
	case 9:
	}
	if k == kinds.Alias {
	}
	if k == kinds.KindA {
	}
	if k == 0 {
	}
	if l == 0 {
	}
	if l == kinds.LevelNone {
	}
	if m == "nope" {
	}
	if m == kinds.ModeRead {
	}
	if m == "" {
	}
}
