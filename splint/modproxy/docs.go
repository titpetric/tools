// Package modproxy queries the Go module proxy about a dependency.
//
// It answers four things a document cannot: how big the module is, the
// publish date of the version in use, what the latest one is, and what that
// version itself requires. All four come from the proxy protocol without
// downloading anything: the size is the Content-Length of a HEAD on the zip,
// and the rest is one small document each.
//
// A report needs nothing from here. A machine with no network answers
// nothing and says so, and the caller reports what the document told it.
//
// New reads a GOPROXY listing more than one proxy down to the first real URL:
// the fallbacks are what the toolchain does when a download fails, and the
// client puts one question that either answers or does not.
//
// A version is immutable, so its size is a fact that does not expire and is
// worth keeping: a tree of forty dependencies is forty requests the next run
// does not make. The Cache holds nothing else the proxy answers. A published
// date is immutable too and a report reads it once; the latest version is
// not immutable at all. Modules do not change size much from one release to
// the next, and an answer within a few percent is worth more than a round
// trip.
package modproxy
