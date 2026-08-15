package main

import "testing"

// TestSpeak holds the only references to Speaker, Threshold, Registry,
// and Mode, and the only conversion of Dog to an interface — the selection
// through Speaker is what dispatch-credits Dog.Speak.
func TestSpeak(t *testing.T) {
	var s Speaker = Dog{}
	if s.Speak() != "woof" {
		t.Fatal("wrong sound")
	}
	if Threshold < 0 || Registry == nil || Mode(0) != 0 {
		t.Fatal("unused")
	}
}

// testHelper is dead in the full view too, so its verdict stays plain.
func testHelper() int { return 1 }
