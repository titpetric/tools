package ast

import (
	"reflect"
	"strings"
)

// JSONTag is the json key of a struct tag.
func JSONTag(tag string) string {
	return reflect.StructTag(tag).Get("json")
}

// JSONTagName is the name of a json tag, without its options.
func JSONTagName(tag string) string {
	out := JSONTag(tag)
	parts := strings.SplitN(out, ",", 2)
	return parts[0]
}

// DBTag is the db key of a struct tag.
func DBTag(tag string) string {
	return reflect.StructTag(tag).Get("db")
}

// DBTagName is the name of a db tag, without its options.
func DBTagName(tag string) string {
	out := DBTag(tag)
	parts := strings.SplitN(out, ",", 2)
	return parts[0]
}
