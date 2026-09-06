// Command imports is the fixture the import fixer is tested against. Its
// imports are written the way a file that has never been formatted writes
// them.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	_ "example.com/imports/client"

	"example.com/imports/service1/model"
)

func main() {
	user := model.User{Name: strings.TrimSpace(os.Args[1])}
	out, _ := json.Marshal(user)
	fmt.Println(string(out))
}
