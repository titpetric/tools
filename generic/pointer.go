package generic

type Value any

func Pointer[T Value](val T) *T {
	return &val
}
