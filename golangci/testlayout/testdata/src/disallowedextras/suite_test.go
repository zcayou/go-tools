// A suite file holding what a suite file may not, in a file of a kind that may
// not exist: only the kind is reported.
package disallowedextras // want `test-package: whitebox test files are not allowed; declare package disallowedextras_test`

const unwanted = 1

func helper() int { return unwanted }
