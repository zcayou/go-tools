package exemptmarkers

// The suite runs this package with "+kubebuilder" configured as an exempt
// prefix. Nothing carrying it is reported, and the prose that does not carry
// it still is, which is what says the setting is read rather than assumed.

// want +2 `comment-wrap: .* over 2 lines; .* takes 1`

// ordinary prose
// still fills
type Ordinary struct{}

// A marker line is left where it was put, and does not join the sentence above
// it either.
// +kubebuilder:validation:Required
// +kubebuilder:validation:MaxLength=63
type Marked struct{}

// A paragraph naming +kubebuilder:object:root in passing is left alone whole,
// because a fill could put that word at the start of a line and make it a
// marker controller-gen would read.
type Mentioned struct{}
