package unpinned // want package:"pins"

type ErrorTyped struct{} // want ErrorTyped:"rendering"

func (*ErrorTyped) Error() string { return "error typed" }

type Prefixed struct{} // want Prefixed:"rendering"

func (Prefixed) Error() string { return "prefixed" }

type Negated struct{} // want Negated:"rendering"

func (Negated) Error() string { return "negated" }

type Ored struct{} // want Ored:"rendering"

func (Ored) Error() string { return "ored" }

type Elsewhere struct{} // want Elsewhere:"rendering"

func (Elsewhere) Error() string { return "elsewhere" }

type Busy struct{} // want Busy:"rendering"

func (Busy) Error() string { panic("busy") }

type Calm struct{} // want Calm:"rendering"

func (Calm) Error() string { return "calm" }
