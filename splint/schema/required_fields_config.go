package schema

// RequiredFieldsConfig defines which fields are required for each type.
type RequiredFieldsConfig struct {
	Fields map[string][]string
}

// NewDefaultConfig just returns a sample required-fields config
func NewDefaultConfig() *RequiredFieldsConfig {
	return &RequiredFieldsConfig{
		Fields: map[string][]string{
			//"User":  {"ID", "Name"}, // Only ID and Name are required for User
			//"Inner": {"Name"},
		},
	}
}
