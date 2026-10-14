package visibility

// Each enum below is unexported and declares no name, so unnamed
// reports exactly the ones something makes visible.

// private is met by nothing outside the package.
type private int

const privateA private = 1

func keepPrivate() private { return privateA }

// field is the type of an exported field of an exported type.
type field int

const fieldA field = 1

// hidden is the type of an unexported field of the same type.
type hidden int

const hiddenA hidden = 1

type Holder struct {
	Field  field
	hidden hidden
}

// variable is the type of an exported variable.
type variable int

const variableA variable = 1

var Current = variableA

// member is the type of an exported member.
type member int

const MemberA member = 1

// param is a parameter of an exported function.
type param int

const paramA param = 1

func Take(param) {}

// result is a result of an exported function.
type result int

const resultA result = 1

func Give() result { return resultA }

// method is a result of an exported method of an exported type.
type method int

const methodA method = 1

type Service struct{}

func (Service) Mode() method { return methodA }

// level is a result of an exported method of an unexported type an exported
// function returns.
type level int

const levelA level = 1

type impl struct{}

func New() *impl { return &impl{} }

func (*impl) Level() level { return levelA }

// unexposed is a result of an exported method of an unexported type nothing
// exposes.
type unexposed int

const unexposedA unexposed = 1

type internal struct{}

func (internal) Unexposed() unexposed { return unexposedA }

// sourced is a result of a method an exported interface declares.
type sourced int

const sourcedA sourced = 1

type Source interface {
	Sourced() sourced
}

// code is an error type.
type code int

const codeA code = 1

func (c code) Error() string { return "code" }

// slot is held by an exported slice type.
type slot int

const slotA slot = 1

type Slots []slot

// signature is a parameter of an exported function type's signature, which
// holds no value of it.
type signature int

const signatureA signature = 1

type Callback func(signature)
