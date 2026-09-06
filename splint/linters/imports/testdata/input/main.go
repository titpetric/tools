// Command imports is the fixture the import fixer is tested against. Its
// imports are written the way a file that has never been formatted writes
// them.
package main

import "fmt"

import (
	"example.com/imports/service1/model"
	"os"
	json "encoding/json"
	_ "example.com/imports/client"
	"strings"
)

func main() {
	user := model.User{Name: strings.TrimSpace(os.Args[1])}
	out, _ := json.Marshal(user)
	fmt.Println(string(out))
}
