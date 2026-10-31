package upfixture

import "os"

func fromNonTestFile() []byte {
	b, _ := os.ReadFile("../../LICENSE")
	return b
}
