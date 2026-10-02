//go:build !darwin

package midi

import "fmt"

type Input struct{}

func Open(func([]byte)) (*Input, error) {
	return nil, fmt.Errorf("midi: native input is currently available on macOS")
}
func (*Input) Close() {}
