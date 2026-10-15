// Package secret is internal, so no caller outside the module can name its
// types.
package secret

type Token struct{ ID int }

func (t Token) String() string { return "token" }
