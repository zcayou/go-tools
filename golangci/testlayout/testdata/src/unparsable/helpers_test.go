package unparsable

func Describe(text string, body func()) bool { body(); return true }
