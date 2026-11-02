//go:build !windows

package main

import "errors"

func newSampler() (Sampler, error) {
	return nil, errors.New("sleepy-client currently supports Windows only")
}
