package simpleparser

import (
	"bytes"
	"go/build/constraint"
	"runtime"
	"strings"
)

// Included reports whether a file is one this build reads.
//
// Two things exclude a file: a name ending in a platform the build is not for,
// "handler_windows.go" on linux, and a build constraint that evaluates false.
// Both are what the toolchain applies before a compiler ever sees the file, so
// a parser that read them anyway would describe a package nobody builds.
//
// The constraint is evaluated with go/build/constraint, which parses the
// expression without building anything: a tag other than the platform is
// treated as unset, which is what a plain "go build" does with it.
func Included(name string, source []byte) bool {
	if !platformMatches(name) {
		return false
	}

	expr := constraintOf(source)
	if expr == nil {
		return true
	}
	return expr.Eval(satisfied)
}

// constraintOf returns the build constraint a file carries, and nil for a file
// with none. The lines before the package clause are the only ones that can
// hold it.
func constraintOf(source []byte) constraint.Expr {
	var plusBuild constraint.Expr

	// Only the header is read. Splitting the whole file to find a line that
	// has to be in the first few is most of the cost of reading one.
	for rest := source; len(rest) > 0; {
		var line []byte
		if end := bytes.IndexByte(rest, '\n'); end >= 0 {
			line, rest = rest[:end], rest[end+1:]
		} else {
			line, rest = rest, nil
		}

		trimmed := strings.TrimSpace(string(line))
		if strings.HasPrefix(trimmed, "package ") {
			break
		}
		if !strings.HasPrefix(trimmed, "//") {
			continue
		}

		// The //go:build line wins outright, which is what the toolchain does
		// with a file carrying both.
		if expr, err := constraint.Parse(trimmed); err == nil {
			if constraint.IsGoBuild(trimmed) {
				return expr
			}
			if plusBuild == nil {
				plusBuild = expr
			} else {
				plusBuild = &constraint.AndExpr{X: plusBuild, Y: expr}
			}
		}
	}

	return plusBuild
}

// platformMatches reports whether a filename names a platform this build is
// for. A name ending in "_linux" or "_amd64" is for that platform alone.
func platformMatches(name string) bool {
	base := strings.TrimSuffix(name, ".go")
	base = strings.TrimSuffix(base, "_test")

	parts := strings.Split(base, "_")
	if len(parts) < 2 {
		return true
	}

	// The last two elements may be "_GOOS_GOARCH"; either alone is the last.
	last := parts[len(parts)-1]
	if isArch(last) {
		if !satisfied(last) {
			return false
		}
		if len(parts) < 3 {
			return true
		}
		last = parts[len(parts)-2]
	}
	if isOS(last) {
		return satisfied(last)
	}

	return true
}

// satisfied reports whether a build tag holds for this build.
//
// The platform tags answer for themselves and "unix" for the platforms it
// covers; every other tag is unset, which is what a plain build leaves it.
func satisfied(tag string) bool {
	switch tag {
	case runtime.GOOS, runtime.GOARCH:
		return true
	case "unix":
		return isUnix(runtime.GOOS)
	case "gc":
		return true
	}

	if strings.HasPrefix(tag, "go1.") {
		// The language version this is built with is at least the one the
		// module declares, so a version tag holds.
		return true
	}

	return false
}

// isUnix reports whether an operating system is one the unix tag covers.
func isUnix(goos string) bool {
	switch goos {
	case "aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos",
		"ios", "linux", "netbsd", "openbsd", "solaris":
		return true
	}
	return false
}

// isOS reports whether a word names an operating system.
func isOS(word string) bool {
	switch word {
	case "aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos",
		"ios", "js", "linux", "nacl", "netbsd", "openbsd", "plan9", "solaris",
		"wasip1", "windows", "zos":
		return true
	}
	return false
}

// isArch reports whether a word names an architecture.
func isArch(word string) bool {
	switch word {
	case "386", "amd64", "amd64p32", "arm", "arm64", "arm64be", "armbe",
		"loong64", "mips", "mips64", "mips64le", "mips64p32", "mips64p32le",
		"mipsle", "ppc", "ppc64", "ppc64le", "riscv", "riscv64", "s390",
		"s390x", "sparc", "sparc64", "wasm":
		return true
	}
	return false
}
