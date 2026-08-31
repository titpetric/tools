// Package visible keeps most of itself private, which is what the visibility
// check measures.
package visible

// Do is the one thing this package offers.
func Do(n int) int {
	return step(step(step(n)))
}

func step(n int) int {
	if n%2 == 0 {
		return n / 2
	}
	return 3*n + 1
}

func unused(n int) int {
	total := 0
	for i := 0; i < n; i++ {
		if i%3 == 0 {
			total += i
		}
		if i%5 == 0 {
			total -= i
		}
	}
	return total
}

func alsoUnused(values []int) int {
	high := 0
	for _, value := range values {
		if value > high {
			high = value
		}
	}
	return high
}
