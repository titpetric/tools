package model

type Assets struct {
	data map[string]string
}

func (a *Assets) Get(key string) string {
	return a.data[key]
}
