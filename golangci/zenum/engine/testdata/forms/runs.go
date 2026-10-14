package forms

//go:generate go run golang.org/x/tools/cmd/stringer -type=Single,Runs,Sparse -output=runs_string.go

// Single is one run of values, which stringer renders from one table.
type Single int

const (
	SingleA Single = iota + 1
	SingleB
	SingleC
)

// Runs is a few runs of values, which stringer renders from one table each.
type Runs uint8

const (
	RunsA Runs = 1
	RunsB Runs = 2
	RunsC Runs = 7
	RunsD Runs = 8
	RunsE Runs = 20
)

// Sparse is more runs than stringer renders from tables, so it renders
// from a map.
type Sparse int

const (
	Sparse1  Sparse = 1
	Sparse3  Sparse = 3
	Sparse5  Sparse = 5
	Sparse7  Sparse = 7
	Sparse9  Sparse = 9
	Sparse11 Sparse = 11
	Sparse13 Sparse = 13
	Sparse15 Sparse = 15
	Sparse17 Sparse = 17
	Sparse19 Sparse = 19
	Sparse21 Sparse = 21
	Sparse23 Sparse = 23
)
