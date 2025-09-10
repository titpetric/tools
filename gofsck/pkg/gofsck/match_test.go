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
			"service*.go",
			"default.go",
		}
		want = append(want, allowlist...)

		assert.Equal(t, want, got)
	})

	t.Run("Global", func(t *testing.T) {
		got := matchFilenames("Get", "", "default.go")
		want := []string{
			"get.go",
			"default.go",
		}
		want = append(want, allowlist...)

		assert.Equal(t, want, got)
	})

	t.Run("Plurality", func(t *testing.T) {
		got := matchFilenames("Get", "Assets", "default.go")
		want := []string{
			"assets_get.go",
			"assets*.go",
			"asset*.go",
			"default.go",
		}
		want = append(want, allowlist...)

		assert.Equal(t, want, got)
	})

	t.Run("Doer", func(t *testing.T) {
		got := matchFilenames("Do", "Checker", "default.go")
		want := []string{
			"checker_do.go",
			"checker*.go",
			"check*.go",
			"default.go",
		}
		want = append(want, allowlist...)

		assert.Equal(t, want, got)
	})

	t.Run("Global bound", func(t *testing.T) {
		got := matchFilenames("LimiterFunc", "", "default.go")
		want := []string{
			"limiter_func.go",
			"limiter*.go",
			"limit*.go",
			"default.go",
		}
		want = append(want, allowlist...)

		assert.Equal(t, want, got)
	})

	t.Run("New", func(t *testing.T) {
		got := matchFilenames("NewSchedulerContextTimeout", "", "default.go")
		want := []string{
			"scheduler_context_timeout.go",
			"scheduler_context*.go",
			"scheduler*.go",
			"schedul*.go",
			"default.go",
		}
		want = append(want, allowlist...)

		assert.Equal(t, want, got)
	})
}
