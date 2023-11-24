// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package plog provides diagnostic output functions according to the Go
// standard logger "log" where log messages are prefixed by the invoker's
// package name and log output can be enabled/disabled programmatically.
package plog

import (
	"log"
	"runtime"
	"strings"
)

var logger *log.Logger = WithPrefix("")
var disabled = false

// WithPrefix creates a *log.Logger with the given message prefix.
func WithPrefix(prefix string) *log.Logger {
	return log.New(log.Default().Writer(), prefix, log.Ldate|log.Ltime|log.Lmicroseconds|log.Lmsgprefix)
}

// Disable disables log output.
func Disable() {
	disabled = true
}

// Enable enables log output.
func Enable() {
	disabled = false
}

// Enabled checks whether log output is currently enabled.
func Enabled() bool {
	return !disabled
}

// Fatal is equivalent to log.Fatal. In addition, the message is prefixed with
// the invoker's package name. Output is never discarded even if logging is
// disabled.
func Fatal(v ...any) {
	logger.SetPrefix(getPackagePrefix())
	logger.Fatal(v...)
}

// Fatalf is equivalent to log.Fatalf. In addition, the message is prefixed with
// the invoker's package name. Output is never discarded even if logging is
// disabled.
func Fatalf(format string, v ...any) {
	logger.SetPrefix(getPackagePrefix())
	logger.Fatalf(format, v...)
}

// Fatalln is equivalent to log.Fatalln. In addition, the message is prefixed
// with the invoker's package name. Output is never discarded even if logging is
// disabled.
func Fatalln(v ...any) {
	logger.SetPrefix(getPackagePrefix())
	logger.Fatalln(v...)
}

// Print is equivalent to log.Print. In addition, the message is prefixed with
// the invoker's package name. Output is discarded if logging is disabled.
func Print(v ...any) {
	if disabled {
		return
	}
	logger.SetPrefix(getPackagePrefix())
	logger.Print(v...)
}

// Printf is equivalent to log.Printf. In addition, the message is prefixed with
// the invoker's package name. Output is discarded if logging is disabled.
func Printf(format string, v ...any) {
	if disabled {
		return
	}
	logger.SetPrefix(getPackagePrefix())
	logger.Printf(format, v...)
}

// Println is equivalent to log.Println. In addition, the message is prefixed
// with the invoker's package name. Output is discarded if logging is disabled.
func Println(v ...any) {
	if disabled {
		return
	}
	logger.SetPrefix(getPackagePrefix())
	logger.Println(v...)
}

func getPackagePrefix() string {
	pc, _, _, _ := runtime.Caller(2)
	parts := strings.Split(runtime.FuncForPC(pc).Name(), ".")
	pl := len(parts)
	pkgPart := ""
	if parts[pl-2][0] == '(' {
		pkgPart = parts[pl-3] // method receiver
	} else {
		pkgPart = parts[pl-2] // plain function
	}
	return pkgPart[strings.LastIndex(pkgPart, "/")+1:] + " "
}
