package schema

// JSONSchema represents a JSON Schema document according to the draft-07 specification.
// It includes standard fields used to define types, formats, validations.
type JSONSchema struct {
	// Schema specifies the JSON Schema version URL.
	// Example: "http://json-schema.org/draft-07/schema#"
	Schema string `json:"$schema,omitempty"`
	// Ref references another schema definition.
	// Example: "#/definitions/SomeType"
	Ref string `json:"$ref,omitempty"`
	// Definitions contains subSchema definitions that $ref can reference.
	Definitions map[string]*JSONSchema `json:"definitions,omitempty"`
	// Type names the JSON kind of the instance (e.g., "object", "array", "string").
	Type string `json:"type,omitempty"`
	// Format adds semantic validation for the instance.
	// Common formats include "date-time", "email", etc.
	Format string `json:"format,omitempty"`
	// Pattern defines a regular expression that a string value must match
	Pattern string `json:"pattern,omitempty"`
	// Properties defines the fields of an object and their corresponding schemas
	Properties map[string]*JSONSchema `json:"properties,omitempty"`
	// Items defines the schema for array elements
	Items *JSONSchema `json:"items,omitempty"`
	// Enum restricts a value to a fixed set of values
	Enum []any `json:"enum,omitempty"`
	// Required lists the properties that must be present in an object
	Required []string `json:"required,omitempty"`
	// Description holds a human-readable explanation of the schema.
	Description string `json:"description,omitempty"`
	// Minimum specifies the lowest numeric value allowed.
	Minimum *float64 `json:"minimum,omitempty"`
	// Maximum specifies the highest numeric value allowed.
	Maximum *float64 `json:"maximum,omitempty"`
	// ExclusiveMinimum, if true, requires the instance to sit strictly above the lower bound.
	ExclusiveMinimum *bool `json:"exclusiveMinimum,omitempty"`
	// ExclusiveMaximum, if true, requires the instance to sit strictly below the upper bound.
	ExclusiveMaximum *bool `json:"exclusiveMaximum,omitempty"`
	// MultipleOf requires the numeric instance to divide evenly by this value.
	MultipleOf *float64 `json:"multipleOf,omitempty"`
	// AdditionalProperties controls whether an object can have properties beyond those defined
	// Can be a boolean or a schema that the extra properties must conform to
	AdditionalProperties any `json:"additionalProperties,omitempty"`
}
