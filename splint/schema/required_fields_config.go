package schema

// RequiredFieldsConfig lists the required fields of each type.
type RequiredFieldsConfig struct {
	Fields map[string][]string
}

// NewDefaultConfig returns a sample required-fields configuration.
func NewDefaultConfig() *RequiredFieldsConfig {
	return &RequiredFieldsConfig{
		Fields: map[string][]string{
			//"User":  {"ID", "Name"}, // User requires only ID and Name
			//"Inner": {"Name"},
		},
	}
}
