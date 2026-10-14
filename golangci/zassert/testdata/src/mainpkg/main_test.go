package main

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestMain_(t *testing.T) {
	Expect(do()).To(HaveOccurred())
}
