//go:build tools

// Package tools pins build-time-only dependencies so `go mod tidy` keeps them.
// golang.org/x/mobile is required by `gomobile bind` (which generates the
// mobile bindings from core/mobile) even though no shipped code imports it.
package tools

import _ "golang.org/x/mobile/bind"
