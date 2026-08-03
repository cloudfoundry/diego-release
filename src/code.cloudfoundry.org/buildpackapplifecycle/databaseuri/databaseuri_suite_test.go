package databaseuri_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDatabaseuri(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Databaseuri Suite")
}
