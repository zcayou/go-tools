package missing

type Excluded struct{} // want Excluded:"rendering"

func (*Excluded) Error() string { return "excluded" } // want `^unpinned-rendering: missing\.Excluded has no excluded_test\.go to pin its rendering in$`
