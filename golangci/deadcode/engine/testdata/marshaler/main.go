package main

import (
	"encoding/json"
	"fmt"
)

// Payload is marshalled through encoding/json, which discovers MarshalJSON with reflect.TypeAssert
// rather than with a type assertion any syntax scan would see.
type Payload struct{ N int }

func (p Payload) MarshalJSON() ([]byte, error) {
	return fmt.Appendf(nil, `{"n":%d}`, p.N), nil
}

func main() {
	out, err := json.Marshal(Payload{N: 1})
	fmt.Println(string(out), err)
}
