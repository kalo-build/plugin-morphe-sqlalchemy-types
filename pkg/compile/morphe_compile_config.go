package compile

import (
	"path"

	rcfg "github.com/kalo-build/morphe-go/pkg/registry/cfg"
	"github.com/kalo-build/plugin-morphe-sqlalchemy-types/pkg/compile/cfg"
)

// MorpheCompileConfig contains all configuration for compiling Morphe to the target format
type MorpheCompileConfig struct {
	// Registry loading configuration
	rcfg.MorpheLoadRegistryConfig

	// Output path for generated files
	OutputPath string

	// Format-specific configuration
	FormatConfig SQLAlchemyConfig

	// Type-specific configuration
	MorpheConfig cfg.MorpheConfig
}

// SQLAlchemyConfig contains SQLAlchemy-specific configuration options
type SQLAlchemyConfig struct {
	// SQLAlchemy-specific options
	UseDeclarative  bool   `json:"useDeclarative"`  // Use declarative base (default: true)
	UseDataclass    bool   `json:"useDataclass"`    // Use dataclass mixin (default: false)
	AddTypeHints    bool   `json:"addTypeHints"`    // Add type hints (default: true)
	GenerateInit    bool   `json:"generateInit"`    // Generate __init__.py files (default: true)
	IndentSize      int    `json:"indentSize"`      // Number of spaces for indent (default: 4)
	PythonVersion   string `json:"pythonVersion"`   // Target Python version (default: "3.8")
	TableNamePrefix string `json:"tableNamePrefix"` // Prefix for table names (default: "")
	TableNameSuffix string `json:"tableNameSuffix"` // Suffix for table names (default: "")
}

// DefaultMorpheCompileConfig creates a default configuration
func DefaultMorpheCompileConfig(
	yamlRegistryPath string,
	baseOutputDirPath string,
) MorpheCompileConfig {
	return MorpheCompileConfig{
		MorpheLoadRegistryConfig: rcfg.MorpheLoadRegistryConfig{
			RegistryEnumsDirPath:      path.Join(yamlRegistryPath, "enums"),
			RegistryModelsDirPath:     path.Join(yamlRegistryPath, "models"),
			RegistryStructuresDirPath: path.Join(yamlRegistryPath, "structures"),
			RegistryEntitiesDirPath:   path.Join(yamlRegistryPath, "entities"),
		},
		OutputPath: baseOutputDirPath,
		FormatConfig: SQLAlchemyConfig{
			UseDeclarative:  true,
			UseDataclass:    false,
			AddTypeHints:    true,
			GenerateInit:    true,
			IndentSize:      4,
			PythonVersion:   "3.8",
			TableNamePrefix: "",
			TableNameSuffix: "",
		},
	}
}

// Validate checks if the configuration is valid
func (config MorpheCompileConfig) Validate() error {
	// Validate registry paths
	if err := config.MorpheLoadRegistryConfig.Validate(); err != nil {
		return err
	}

	// TODO: Add format-specific validation
	// Examples:
	// - Check if package prefix is valid
	// - Verify indent size is positive
	// - Ensure file extension starts with "."

	return nil
}
