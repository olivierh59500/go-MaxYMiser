//go:build !darwin

package midi

import "fmt"

type Output struct{}

func Destinations() ([]Destination, error) {
	return nil, fmt.Errorf("midi: native output is currently available on macOS")
}
func OpenOutput(uint32) (*Output, error) {
	return nil, fmt.Errorf("midi: native output is currently available on macOS")
}
func (*Output) Send([]byte) error { return fmt.Errorf("midi: output is unavailable on this platform") }
func (*Output) Close()            {}
