package project

import "github.com/olivierh59500/go-MaxYMiser/internal/native"

// SaveConfiguration replaces a complete native configuration atomically, so
// saving edited preferences can update the existing MYM.CNF file.
func SaveConfiguration(config native.Configuration, path string) error {
	return atomicWrite(path, config[:])
}
