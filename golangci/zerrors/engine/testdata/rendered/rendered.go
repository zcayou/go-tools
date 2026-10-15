// Package rendered stores renderings into the string fields of error types.
package rendered

import (
	"fmt"

	"example.com/rendered/internal/secret"
	"example.com/rendered/store"
)

type NotFoundError struct{ Key string }

func (e *NotFoundError) Error() string { return e.Key }

type MixedError struct{ Key string }

func (e *MixedError) Error() string { return e.Key }

type TwoTypesError struct{ Name string }

func (e *TwoTypesError) Error() string { return e.Name }

type GenericError struct{ Ref string }

func (e *GenericError) Error() string { return e.Ref }

type TwoRefsError struct{ Ref string }

func (e *TwoRefsError) Error() string { return e.Ref }

type ThroughError struct{ Key string }

func (e *ThroughError) Error() string { return e.Key }

type PathError struct{ Path string }

func (e *PathError) Error() string { return e.Path }

type KindError struct{ Kind string }

func (e *KindError) Error() string { return e.Kind }

type TokenError struct{ Token string }

func (e *TokenError) Error() string { return e.Token }

type LocalError struct{ Key string }

func (e *LocalError) Error() string { return e.Key }

type StringerError struct{ Key string }

func (e *StringerError) Error() string { return e.Key }

type localKey struct{ n int }

func (k localKey) String() string { return "local" }

func Errors(k store.Key, p store.Path, kind store.Kind, t secret.Token, s fmt.Stringer, r store.Ref[int]) []error {
	return []error{
		&NotFoundError{Key: k.String()},
		&MixedError{Key: k.String()},
		&MixedError{Key: "fixed"},
		&TwoTypesError{Name: k.String()},
		&TwoTypesError{Name: r.String()},
		missing(r),
		&TwoRefsError{Ref: r.String()},
		&TwoRefsError{Ref: store.Ref[string]{ID: "x"}.String()},
		through(k),
		&PathError{Path: p.String()},
		&KindError{Kind: kind.String()},
		&TokenError{Token: t.String()},
		&LocalError{Key: localKey{}.String()},
		&StringerError{Key: s.String()},
	}
}

func missing[K interface{ String() string }](k K) error { return &GenericError{Ref: k.String()} }

func through(k store.Key) error {
	name := k.String()
	return &ThroughError{Key: render(name)}
}

func render(s string) string { return s }
