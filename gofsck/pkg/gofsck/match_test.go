package gofsck

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_matchFilenames(t *testing.T) {
	t.Run("With receiver", func(t *testing.T) {
		got := matchFilenames("Get", "ServiceDiscovery", "default.go")
		want := []string{
			"service_discovery_get.go",
			"service_discovery*.go",
			"get.go",
			"default.go",
		}

		assert.Equal(t, want, got)
	})

	t.Run("Global", func(t *testing.T) {
		got := matchFilenames("Get", "", "default.go")
		want := []string{
			"get.go",
			"default.go",
		}

		assert.Equal(t, want, got)
	})

	t.Run("Global bound", func(t *testing.T) {
		got := matchFilenames("LimiterFunc", "", "default.go")
		want := []string{
			"limiter_func.go",
			"limiter*.go",
			"default.go",
		}

		assert.Equal(t, want, got)
	})

	t.Run("New", func(t *testing.T) {
		got := matchFilenames("NewScheduler", "", "default.go")
		want := []string{
			"scheduler.go",
			"new*.go",
			"default.go",
		}

		assert.Equal(t, want, got)
	})
}
