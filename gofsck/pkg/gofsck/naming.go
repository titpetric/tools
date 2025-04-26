package gofsck

import (
	strcase "github.com/stoewer/go-strcase"
)

func toSnake(input string) string {
	return strcase.SnakeCase(input)
}
