module example.com/indirect

go 1.27

require (
	example.com/inner v0.0.0
	example.com/outer v0.0.0
)

replace (
	example.com/inner => ../inner
	example.com/outer => ../outer
)
