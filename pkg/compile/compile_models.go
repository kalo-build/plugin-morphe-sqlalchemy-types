package compile

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kalo-build/morphe-go/pkg/registry"
	"github.com/kalo-build/morphe-go/pkg/yaml"
	"github.com/kalo-build/morphe-go/pkg/yamlops"
	"github.com/kalo-build/plugin-morphe-sqlalchemy-types/pkg/compile/cfg"
	"github.com/kalo-build/plugin-morphe-sqlalchemy-types/pkg/formatdef"
	"github.com/kalo-build/plugin-morphe-sqlalchemy-types/pkg/typemap"
)

// resolvePolymorphicThrough looks up the model that has the polymorphic relationship
func resolvePolymorphicThrough(through string, r *registry.Registry) (string, error) {
	// Find the model that has this polymorphic relationship
	for modelName, model := range r.GetAllModels() {
		for relName, rel := range model.Related {
			if relName == through && yamlops.IsRelationPoly(string(rel.Type)) {
				return modelName, nil
			}
		}
	}
	return "", fmt.Errorf("polymorphic relationship %s not found", through)
}

// CompileModel converts a Morphe model to the target format
func CompileModel(model yaml.Model, r *registry.Registry) (*formatdef.Struct, error) {
	// Create the struct definition
	formatStruct := &formatdef.Struct{
		Name:   model.Name,
		Fields: make([]formatdef.Field, 0),
	}

	// Sort fields for consistent output
	var fieldNames []string
	for name := range model.Fields {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)

	// Add fields
	for _, fieldName := range fieldNames {
		field := model.Fields[fieldName]
		fieldType := typemap.GetFieldType(field.Type)
		formatField := formatdef.Field{
			Name: fieldName,
			Type: fieldType,
		}
		formatStruct.Fields = append(formatStruct.Fields, formatField)
	}

	// Process related models (if any)
	if len(model.Related) > 0 {
		// Sort related for consistent output
		var relatedNames []string
		for name := range model.Related {
			relatedNames = append(relatedNames, name)
		}
		sort.Strings(relatedNames)

		// Add foreign key fields
		for _, relatedName := range relatedNames {
			relation := model.Related[relatedName]
			relationType := string(relation.Type)

			// Handle polymorphic relationships
			if yamlops.IsRelationPoly(relationType) && yamlops.IsRelationFor(relationType) && yamlops.IsRelationOne(relationType) {
				// ForOnePoly: Add type and id fields
				typeField := formatdef.Field{
					Name: formatdef.ToCamelCase(relatedName + "_type"),
					Type: formatdef.TypeString,
				}
				formatStruct.Fields = append(formatStruct.Fields, typeField)

				idField := formatdef.Field{
					Name: formatdef.ToCamelCase(relatedName + "_id"),
					Type: formatdef.TypeString,
				}
				formatStruct.Fields = append(formatStruct.Fields, idField)
			} else if yamlops.IsRelationPoly(relationType) {
				// Other polymorphic types (HasOnePoly, HasManyPoly, ForManyPoly)
				// These don't add fields to the model, but affect how we handle relationships
				continue
			} else if yamlops.IsRelationFor(relationType) && yamlops.IsRelationOne(relationType) {
				// Regular ForOne: Add foreign key field
				relField := formatdef.Field{
					Name: formatdef.ToCamelCase(relatedName + "_id"),
					Type: formatdef.TypeString,
				}
				formatStruct.Fields = append(formatStruct.Fields, relField)
			}
			// HasOne, HasMany, ForMany don't add fields to this model
		}

		// Add navigation properties for relationships (for Python type hints)
		for _, relatedName := range relatedNames {
			relation := model.Related[relatedName]
			relationType := string(relation.Type)

			// Resolve the actual target model name using aliasing
			targetModelName := yamlops.GetRelationTargetName(relatedName, relation.Aliased)

			// Add navigation field based on relationship type
			var navType formatdef.Type
			if yamlops.IsRelationPoly(relationType) {
				// Polymorphic relationships need Union types
				if len(relation.For) > 0 {
					// Create a custom type representing the Union
					unionType := "Union["
					for i, forModel := range relation.For {
						if i > 0 {
							unionType += ", "
						}
						unionType += "'" + forModel + "'"
					}
					unionType += "]"
					navType = formatdef.BasicType{Name: unionType}
				} else if relation.Through != "" {
					// HasManyPoly/HasOnePoly with through - resolve the actual model
					throughModel, err := resolvePolymorphicThrough(relation.Through, r)
					if err != nil {
						// Fallback to Any if we can't resolve
						navType = formatdef.TypeAny
					} else {
						navType = formatdef.BasicType{Name: throughModel}
					}
				} else {
					// No 'for' or 'through' specified, use Any
					navType = formatdef.TypeAny
				}
			} else {
				// Regular relationship
				navType = formatdef.BasicType{Name: targetModelName}
			}

			// Determine if it's a collection
			if yamlops.IsRelationMany(relationType) {
				navType = formatdef.ArrayType{ElementType: navType}
			}

			// Add navigation field (prefixed with _ to distinguish from data fields)
			navField := formatdef.Field{
				Name: "_nav_" + relatedName,
				Type: navType,
			}
			formatStruct.Fields = append(formatStruct.Fields, navField)
		}
	}

	return formatStruct, nil
}

// CompileAllModels compiles all models and writes them using the writer
func CompileAllModels(config MorpheCompileConfig, r *registry.Registry, writer *MorpheWriter) error {
	modelContents := make(map[string][]byte)

	// Process each model in the registry
	for modelName, model := range r.GetAllModels() {
		// Compile the model
		compiledModel, err := CompileModel(model, r)
		if err != nil {
			return fmt.Errorf("failed to compile model %s: %w", modelName, err)
		}

		// Generate the content for this model
		content := generateModelContent(compiledModel, model, config.FormatConfig, config.MorpheConfig, r)
		modelContents[modelName] = content
	}

	// Write all model contents
	return writer.WriteAllModels(modelContents)
}

// generateModelContent generates SQLAlchemy model
func generateModelContent(model *formatdef.Struct, yamlModel yaml.Model, config SQLAlchemyConfig, morpheConfig cfg.MorpheConfig, r *registry.Registry) []byte {
	cb := formatdef.NewContentBuilder("    ")

	// Add header comment
	cb.Line("# Code generated by Morphe")
	cb.Line("# SQLAlchemy model definition")
	if config.UseDeclarative {
		cb.Line("# Note: This requires a Base class defined as:")
		cb.Line("#   from sqlalchemy.ext.declarative import declarative_base")
		cb.Line("#   Base = declarative_base()")
	}
	cb.Line("")

	// Create import tracker
	imports := NewImportTracker(r)

	// Add SQLAlchemy imports
	if config.UseDeclarative {
		imports.AddSQLAlchemy("Column", "Integer", "String", "Text", "Float", "Boolean", "DateTime", "Date", "ForeignKey", "JSON")
		imports.AddSQLAlchemy("relationship")
		// Note: Base needs to be imported from a module that defines it
		// In a typical SQLAlchemy project, this would be defined as:
		// Base = declarative_base()
		// For now, we'll add a comment explaining this
		imports.AddFrom(".base", "Base")
	}

	// Track whether we have polymorphic fields
	hasPolymorphicTypeField := false
	polymorphicTypeToNavMap := make(map[string]string)
	hasEnumField := false

	// Scan all fields to determine imports
	for _, field := range model.Fields {
		// Skip navigation properties
		if strings.HasPrefix(field.Name, "_nav_") {
			continue
		}

		typeName := field.Type.GetName()
		imports.TrackFieldType(typeName)

		// Check if this field is an enum
		if basicType, ok := field.Type.(formatdef.BasicType); ok {
			innerType := extractInnerType(basicType.Name)
			if innerType != "" && resolveFieldType(innerType, r) == "enum" {
				// Enum fields are handled differently in SQLAlchemy
				hasEnumField = true
			}
		}

		// Check for polymorphic type fields
		if strings.HasSuffix(field.Name, "_type") && typeName == "str" {
			// Look for corresponding nav field
			navFieldName := "_nav_" + strings.TrimSuffix(field.Name, "_type")
			polymorphicTypeToNavMap[field.Name] = navFieldName
			hasPolymorphicTypeField = true
		}
	}

	// Scan navigation properties
	for _, field := range model.Fields {
		if !strings.HasPrefix(field.Name, "_nav_") {
			continue
		}

		typeName := field.Type.GetName()
		imports.TrackFieldType(typeName)
	}

	// We always need Optional for navigation properties
	if config.AddTypeHints {
		imports.AddTyping("Optional")
	}

	// Add Literal if we have polymorphic type fields
	if hasPolymorphicTypeField {
		imports.AddTyping("Literal")
	}

	// Add Enum if we have enum fields
	if hasEnumField && config.UseDeclarative {
		imports.AddSQLAlchemy("Enum")
	}

	// Generate imports
	imports.Generate(cb)
	cb.Line("")

	// Generate class
	if config.UseDeclarative {
		cb.Line("class %s(Base):", model.Name)
		cb.Indent()
		// Add table name
		cb.Line("__tablename__ = '%s%s%s'", config.TableNamePrefix, formatdef.ToSnakeCase(model.Name), config.TableNameSuffix)
		cb.Line("")
	} else {
		cb.Line("class %s:", model.Name)
		cb.Indent()
	}

	// Add docstring
	cb.Line(`"""%s model."""`, model.Name)

	if len(model.Fields) == 0 {
		cb.Line("pass")
	} else {
		// Add fields
		for _, field := range model.Fields {
			// Skip navigation properties
			if strings.HasPrefix(field.Name, "_nav_") {
				continue
			}

			fieldName := SanitizePythonIdentifier(formatdef.ToSnakeCase(field.Name))
			fieldType := field.Type.GetName()

			// Generate SQLAlchemy column definition
			if config.UseDeclarative {
				// Determine if this is a primary key
				isPrimaryKey := false
				if primaryId, exists := yamlModel.Identifiers["primary"]; exists {
					for _, idField := range primaryId.Fields {
						if idField == field.Name {
							isPrimaryKey = true
							break
						}
					}
				}

				// Check if this is a foreign key
				isForeignKey := strings.HasSuffix(fieldName, "_id") && len(fieldName) > 3

				// Check if field is nullable
				nullable := !isPrimaryKey && field.Type.IsNullable()

				// For foreign keys
				if isForeignKey {
					// Try to determine the referenced table
					refFieldName := fieldName[:len(fieldName)-3] // Remove _id suffix
					refTableName := formatdef.ToSnakeCase(refFieldName)
					nullableStr := "False"
					if nullable {
						nullableStr = "True"
					}
					cb.Line("%s = Column(Integer, ForeignKey('%s.id'), nullable=%s)", fieldName, refTableName, nullableStr)
				} else if strings.HasSuffix(fieldName, "_type") {
					// Polymorphic type field
					nullableStr := "False"
					if nullable {
						nullableStr = "True"
					}
					cb.Line("%s = Column(String, nullable=%s)", fieldName, nullableStr)
				} else {
					// Regular column
					sqlType := mapFieldTypeToSQLAlchemy(field.Type)

					// Check if this is an enum field
					if basicType, ok := field.Type.(formatdef.BasicType); ok {
						innerType := extractInnerType(basicType.Name)
						if innerType != "" && resolveFieldType(innerType, r) == "enum" {
							// It's an enum field - use the enum type directly
							if isPrimaryKey {
								cb.Line("%s = Column(Enum(%s), primary_key=True)", fieldName, innerType)
							} else {
								nullableStr := "False"
								if nullable {
									nullableStr = "True"
								}
								cb.Line("%s = Column(Enum(%s), nullable=%s)", fieldName, innerType, nullableStr)
							}
							continue
						}
					}

					// Check if it's an auto-increment field by looking at the original yaml model
					isAutoIncrement := false
					for origFieldName, origField := range yamlModel.Fields {
						if origFieldName == field.Name && origField.Type == yaml.ModelFieldTypeAutoIncrement {
							isAutoIncrement = true
							break
						}
					}

					if isPrimaryKey {
						if isAutoIncrement {
							cb.Line("%s = Column(%s, primary_key=True, autoincrement=True)", fieldName, sqlType)
						} else {
							cb.Line("%s = Column(%s, primary_key=True)", fieldName, sqlType)
						}
					} else {
						nullableStr := "False"
						if nullable {
							nullableStr = "True"
						}
						cb.Line("%s = Column(%s, nullable=%s)", fieldName, sqlType, nullableStr)
					}
				}
			} else {
				// Non-declarative style (fallback)
				cb.Line("%s: %s", fieldName, fieldType)
			}
		}

		// Add navigation properties (relationships) for SQLAlchemy
		if config.UseDeclarative && len(model.Fields) > 0 {
			cb.Line("") // Add blank line before relationships

			for _, field := range model.Fields {
				if !strings.HasPrefix(field.Name, "_nav_") {
					continue
				}

				// Remove _nav_ prefix to get the actual relationship name
				relName := strings.TrimPrefix(field.Name, "_nav_")
				fieldName := SanitizePythonIdentifier(formatdef.ToSnakeCase(relName))
				fieldType := field.Type.GetName()

				// Skip if this is a polymorphic relationship with corresponding type/id fields
				hasPolyFields := false
				for _, f := range model.Fields {
					if f.Name == relName+"_type" || f.Name == relName+"_id" {
						hasPolyFields = true
						break
					}
				}

				if hasPolyFields {
					// For polymorphic relationships, we'll need special handling
					// This would use polymorphic relationships in SQLAlchemy
					continue
				}

				// For regular relationships using SQLAlchemy
				if strings.HasPrefix(fieldType, "List[") {
					// Many relationship - extract the target model name
					targetModel := fieldType[5 : len(fieldType)-1] // Remove List[ and ]
					targetModel = strings.Trim(targetModel, "'\"") // Remove quotes if any
					cb.Line("%s = relationship(\"%s\", back_populates=\"%s\")",
						fieldName, targetModel, formatdef.ToSnakeCase(model.Name))
				} else if strings.Contains(fieldType, "Union[") {
					// Polymorphic union type - skip for now
					continue
				} else {
					// One relationship
					targetModel := strings.Trim(fieldType, "'\"") // Remove quotes if any
					cb.Line("%s = relationship(\"%s\", back_populates=\"%s\")",
						fieldName, targetModel, formatdef.ToSnakeCase(model.Name))
				}
			}
		}

	}

	cb.Dedent() // End of class body

	return cb.Build()
}

// getSQLAlchemyType converts a Python type to SQLAlchemy column type
func getSQLAlchemyType(pythonType string) string {
	// Remove Optional wrapper if present
	if strings.HasPrefix(pythonType, "Optional[") && strings.HasSuffix(pythonType, "]") {
		pythonType = pythonType[9 : len(pythonType)-1]
	}

	switch pythonType {
	case "str":
		return "String"
	case "int":
		return "Integer"
	case "float":
		return "Float"
	case "bool":
		return "Boolean"
	case "datetime":
		return "DateTime"
	case "Dict[str, Any]":
		return "JSON"
	default:
		// Assume it's an enum or foreign key reference
		if strings.Contains(pythonType, ".") {
			// It's an enum
			return "Enum"
		}
		return "String"
	}
}

// mapFieldTypeToSQLAlchemy maps a formatdef.Type to SQLAlchemy column type
func mapFieldTypeToSQLAlchemy(fieldType formatdef.Type) string {
	typeName := fieldType.GetName()
	// Special handling for specific field types
	if typeName == "int" {
		// Check if this might be an auto-increment field
		// In a real implementation, we'd check the original Morphe field type
		return "Integer"
	}
	return getSQLAlchemyType(typeName)
}
