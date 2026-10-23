package api_test

import (
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"example.com/fixture/api"
	"example.com/fixture/store"
)

func TestUnit(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.WriteString(api.Handle())
}

func TestServer(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
}

func TestStore(t *testing.T) {
	_ = store.New("x.db")
}

func TestExec(t *testing.T) {
	_ = exec.Command("sh", "-c", "true").Run()
}

func TestHelper(t *testing.T) {
	_ = openMemory()
}

func TestStart(t *testing.T) {
	_ = api.Start()
}

func TestPure(t *testing.T) {
	_ = label()
}
