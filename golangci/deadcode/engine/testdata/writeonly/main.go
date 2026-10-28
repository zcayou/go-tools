package main

import (
	"writeonly/api"
	"writeonly/store"
)

func main() {
	store.Build()
	api.Open()
}
