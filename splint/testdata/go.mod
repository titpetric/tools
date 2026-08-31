module example.com/fixture

go 1.27.0

// Two majors of one module, which link both in and whose types do not satisfy
// each other's interfaces.
require (
	example.com/two v1.4.0
	example.com/two/v2 v2.1.0
	example.com/unused v0.3.0
)

// A replace, which is what makes a build resolve to something the go.mod does
// not say. Nothing here is fetched: the fixture is read, never built.
replace example.com/two => ../local/two
