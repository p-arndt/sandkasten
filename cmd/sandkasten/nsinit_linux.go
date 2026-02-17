//go:build linux

package main

import "github.com/p-arndt/sandkasten/internal/runtime/linux"

func isNsinit() bool {
	return linux.IsNsinit()
}

func runNsinit() error {
	return linux.RunNsinit()
}
