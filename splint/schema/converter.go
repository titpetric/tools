package schema

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Write renders a document as a JSON Schema.
//
// Every package of the document contributes its types, so a schema of a tree
// describes the whole of it rather than one package. A type is named by itself
// where that is unambiguous and by its package where it is not.
func Write(w io.Writer, root *model.DocumentRoot, options Options) error {
	schema, err := Convert(root, options)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	return encoder.Encode(schema)
}

// Convert renders a document as a JSON Schema document.
func Convert(root *model.DocumentRoot, options Options) (*JSONSchema, error) {
	out := &JSONSchema{
		Schema:      "http://json-schema.org/draft-07/schema#",
		Definitions: make(map[string]*JSONSchema),
	}

	config := NewDefaultConfig()
	for _, def := range root.Packages {
		if def.TestPackage {
			continue
		}
		if err := convertPackage(out.Definitions, def, config, options); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// convertPackage adds the types of one package to the definitions.
//
// A name two packages both declare is qualified with the package, since a
// schema has one namespace and two Config types are not the same thing.
func convertPackage(definitions map[string]*JSONSchema, def *model.Definition, config *RequiredFieldsConfig, options Options) error {
	for _, decl := range def.Types {
		if decl.Type == "interface" {
			continue
		}

		schema := generateTypeSchema(decl, config, "", options.StripPrefix)
		if schema == nil {
			continue
		}
		if decl.Doc != "" {
			schema.Description = decl.Doc
		}

		name := decl.Name
		if _, taken := definitions[name]; taken {
			name = def.Package.Package + "." + decl.Name
		}
		definitions[name] = schema
	}

	return nil
}

func generateTypeSchema(decl *model.Declaration, config *RequiredFieldsConfig, pkgName string, stripPrefix []string) *JSONSchema {
	switch {
	case len(decl.Fields) > 0:
		return GenerateStructSchema(decl, config, pkgName, stripPrefix)

	case strings.HasPrefix(decl.Type, "map["):
		return GenerateMapDefinition(decl.Type)

	case strings.HasPrefix(decl.Type, "[]"):
		return GenerateSliceDefinition(decl.Type)

	case !isCustomType(decl.Type):
		return &JSONSchema{Type: getBaseJSONType(decl.Type)}

	case strings.Contains(decl.Type, "."):
		// A type defined as one from another package is described where that
		// package is described, not here.
		return nil

	case decl.Name != decl.Type && len(decl.Fields) == 0:
		refName := getRefName(decl.Type, pkgName, stripPrefix)
		return &JSONSchema{Ref: "#/definitions/" + refName}

	default:
		// A named type is described by what it is defined as, which is
		// not a shape a schema can hold.
		return nil
	}
}

func generateFieldSchema(decl *model.Field, config *RequiredFieldsConfig, pkgName string, stripPrefix []string) *JSONSchema {
	switch {
	case strings.HasPrefix(decl.Type, "map["):
		return GenerateMapDefinition(decl.Type)

	case strings.HasPrefix(decl.Type, "[]"):
		return GenerateSliceDefinition(decl.Type)

	case !isCustomType(decl.Type):
		return &JSONSchema{Type: getBaseJSONType(decl.Type)}

	default:
		// A named type is described by what it is defined as, which is
		// not a shape a schema can hold.
		return nil
	}
}

// CollectTypeDefinitionDeps inspects a named type's underlying type
// (e.g. "CertsData" -> "[]CertData") to find further dependencies.
func CollectTypeDefinitionDeps(typeInfo *model.Declaration, pkgInfo *model.Definition, dependencies map[string]bool) {
	underlying := typeInfo.Type
	// If it's a slice: e.g. "[]CertData"
	if strings.HasPrefix(underlying, "[]") {
		elemType := strings.TrimPrefix(underlying, "[]")
		elemType = strings.TrimPrefix(elemType, "*")
		if isCustomType(elemType) {
			if !dependencies[elemType] {
				dependencies[elemType] = true
				if !strings.Contains(elemType, ".") {
					for _, depType := range pkgInfo.Types {
						if depType.Name == elemType {
							if depType.Type != "" && depType.Name != depType.Type {
								dependencies[depType.Type] = true
							}
							CollectTypeDefinitionDeps(depType, pkgInfo, dependencies)
						}
					}
				}
			}
		}
		return
	}

	// If it's a map: e.g. "map[string]PortWhiteList"
	if strings.HasPrefix(underlying, "map[") {
		handleMapField(underlying, pkgInfo, dependencies)
		return
	}

	if len(typeInfo.Fields) > 0 {
		for _, field := range typeInfo.Fields {
			if strings.HasPrefix(field.Type, "map[") {
				handleMapField(field.Type, pkgInfo, dependencies)
				continue
			}
			baseType := getBaseType(field.Type)
			if isCustomType(baseType) {
				if !dependencies[baseType] {
					dependencies[baseType] = true
					if !strings.Contains(baseType, ".") {
						for _, depType := range pkgInfo.Types {
							if depType.Name == baseType {
								if depType.Type != "" && depType.Name != depType.Type {
									dependencies[depType.Type] = true
								}
								CollectTypeDefinitionDeps(depType, pkgInfo, dependencies)
							}
						}
					}
				}
			}
		}
		return
	}
}

// GenerateStructSchema creates a JSON Schema definition for a struct type.
func GenerateStructSchema(typeInfo *model.Declaration, config *RequiredFieldsConfig, pkgName string, stripPrefix []string) *JSONSchema {
	result := &JSONSchema{
		Type:                 "object",
		Properties:           make(map[string]*JSONSchema),
		AdditionalProperties: false,
	}
	requiredFields := config.Fields[typeInfo.Name]
	requiredMap := make(map[string]bool)
	for _, field := range requiredFields {
		requiredMap[field] = true
	}
	var required []string

	for _, field := range typeInfo.Fields {
		if field.JSONName == "-" || field.JSONName == "" {
			continue
		}

		schema := generateFieldSchema(field, config, pkgName, stripPrefix)
		if schema == nil {
			continue
		}

		if field.Doc != "" {
			schema.Description = field.Doc
		}
		cleanedJson := parseJSONTag(field.JSONName)

		result.Properties[cleanedJson] = schema

		if requiredMap[field.Name] {
			required = append(required, cleanedJson)
		}
	}
	if len(required) > 0 {
		result.Required = required
	}
	return result
}

// GenerateMapDefinition creates a top-level JSON Schema definition for a map type (e.g. map[string]Something).
func GenerateMapDefinition(goType string) *JSONSchema {
	// Example: "map[string]interface{}" or "map[string]PortWhiteList"
	inside := goType[len("map["):]
	parts := strings.SplitN(inside, "]", 2)

	if len(parts) != 2 {
		return &JSONSchema{
			Type:                 "object",
			AdditionalProperties: true,
		}
	}

	keyType := strings.TrimSpace(parts[0])   // e.g. "string"
	valueType := strings.TrimSpace(parts[1]) // e.g. "interface{}" or "PortWhiteList"

	if keyType != "string" {
		return &JSONSchema{
			Type:                 "object",
			AdditionalProperties: true,
		}
	}

	if valueType == "interface{}" || valueType == "any" {
		return &JSONSchema{
			Type:                 "object",
			AdditionalProperties: true,
		}
	}

	if !isCustomType(valueType) {
		return &JSONSchema{
			Type: "object",
			AdditionalProperties: &JSONSchema{
				Type: getBaseJSONType(valueType),
			},
		}
	}

	return &JSONSchema{
		Type: "object",
		AdditionalProperties: &JSONSchema{
			Ref: "#/definitions/" + valueType,
		},
	}
}

// GenerateSliceDefinition creates a top-level JSON Schema definition for a slice type (e.g. []CertData).
func GenerateSliceDefinition(goType string) *JSONSchema {
	elemType := strings.TrimPrefix(goType, "[]")
	elemType = strings.TrimSpace(elemType)

	if !isCustomType(elemType) {
		return &JSONSchema{
			Type: "array",
			Items: &JSONSchema{
				Type: getBaseJSONType(elemType),
			},
		}
	}

	return &JSONSchema{
		Type: "array",
		Items: &JSONSchema{
			Ref: "#/definitions/" + elemType,
		},
	}
}
