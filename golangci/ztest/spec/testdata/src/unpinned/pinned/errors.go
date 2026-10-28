package pinned // want package:"pins"

type Literal struct{} // want Literal:"rendering"

func (Literal) Error() string { return "literal" }

type Variable struct{ Code int } // want Variable:"rendering"

func (*Variable) Error() string { return "variable" }

type Parameter struct{} // want Parameter:"rendering"

func (*Parameter) Error() string { return "parameter" }

type Tabled struct{} // want Tabled:"rendering"

func (*Tabled) Error() string { return "tabled" }

type Asserted struct{} // want Asserted:"rendering"

func (*Asserted) Error() string { return "asserted" }

type Switched struct{} // want Switched:"rendering"

func (*Switched) Error() string { return "switched" }

type Matched struct{} // want Matched:"rendering"

func (*Matched) Error() string { return "matched" }

type Composed struct{} // want Composed:"rendering"

func (Composed) Error() string { return "composed" }

type Generic[T any] struct{ Value T } // want Generic:"rendering"

func (Generic[T]) Error() string { return "generic" }

type Equaled struct{} // want Equaled:"rendering"

func (Equaled) Error() string { return "equaled" }

type Reversed struct{} // want Reversed:"rendering"

func (Reversed) Error() string { return "reversed" }

type Exact struct{} // want Exact:"rendering"

func (Exact) Error() string { return "exact" }

type EqualedError struct{} // want EqualedError:"rendering"

func (*EqualedError) Error() string { return "equaled error" }

type Formatted struct{} // want Formatted:"rendering"

func (Formatted) Error() string { return "formatted" }

type Required struct{} // want Required:"rendering"

func (*Required) Error() string { return "required" }

type Method struct{} // want Method:"rendering"

func (Method) Error() string { return "method" }

type Panicking struct{} // want Panicking:"rendering"

func (Panicking) Error() string { panic("panicking") }

type PanickingWith struct{} // want PanickingWith:"rendering"

func (*PanickingWith) Error() string { panic("panicking with") }

type Hostile struct{} // want Hostile:"rendering"

func (Hostile) Error() string { panic("hostile") }

type HostileValue struct{} // want HostileValue:"rendering"

func (HostileValue) Error() string { panic("hostile value") }

func Do(kind int) error {
	switch kind {
	case 1:
		return &Asserted{}
	default:
		return &Switched{}
	}
}
