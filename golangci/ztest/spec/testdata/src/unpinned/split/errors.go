package split // want package:"pins"

type Neither struct{} // want Neither:"rendering"

func (Neither) Error() string { return "neither" }

type InPackage struct{} // want InPackage:"rendering"

func (InPackage) Error() string { return "in package" }
