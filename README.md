# Morphe SQLAlchemy Types Plugin

A Morphe compilation plugin that generates Python code with SQLAlchemy ORM models for database persistence.

## Features

- ✅ Generates Python 3.8+ compatible code
- ✅ SQLAlchemy declarative models with proper relationships
- ✅ Full type hints support
- ✅ Automatic `__init__.py` generation
- ✅ Handles enums, models, structures, and entities
- ✅ Foreign key relationships with proper constraints
- ✅ **Polymorphic relationships** (ForOnePoly, HasManyPoly, etc.)
- ✅ **Aliasing support** for custom relationship naming
- ✅ Table name customization with prefix/suffix support

## Generated Output Example

### Enum
```python
class Nationality(Enum):
    """Nationality enumeration."""
    D_E = "German"
    F_R = "French"
    U_S = "American"
```

### Model (SQLAlchemy)
```python
class Person(Base):
    __tablename__ = 'person'
    
    """Person model."""
    id = Column(Integer, primary_key=True)
    first_name = Column(String, nullable=False)
    last_name = Column(String, nullable=False)
    nationality = Column(Enum(Nationality), nullable=False)
    company_id = Column(Integer, ForeignKey('company.id'), nullable=True)
    
    # Relationships
    company = relationship("Company", back_populates="persons")
```

### Entity View
```python
class CompanyView:
    """Company entity."""
    id: int  # primary identifier
    name: str
    tax_id: str
    persons: List[Person] = None
```

### Polymorphic Model
```python
class Comment(Base):
    __tablename__ = 'comment'
    
    """Comment model with polymorphic relationship."""
    id = Column(Integer, primary_key=True)
    content = Column(Text, nullable=False)
    commentable_type = Column(String, nullable=True)
    commentable_id = Column(Integer, nullable=True)
```

## Usage

```bash
# Build the plugin
go build ./cmd/plugin

# Generate Python code
./plugin '{"inputPath":"./morphe","outputPath":"./output","verbose":true}'
```

## Configuration

The plugin supports comprehensive Python-specific and type-specific options:

```json
{
  "inputPath": "./morphe",
  "outputPath": "./output",
  "config": {
    // Python-specific settings
    "pythonVersion": "3.11",
    "usePydantic": true,
    "pydanticV2": true,
    "addTypeHints": true,
    "generateInit": true,
    "indentSize": 4,
    
    // Type-specific configurations
    "enums": {
      "generateStrMethod": true,
      "useStrEnum": false
    },
    "models": {
      "useField": true,
      "generateExamples": false,
      "useValidators": false
    },
    "structures": {
      "useDataclass": false,
      "generateSlots": false
    },
    "entities": {
      "generateRepository": false,
      "lazyLoadingStyle": "async",
      "includeValidation": false
    }
  }
}
```

See [KALO_CONFIG_EXAMPLE.md](KALO_CONFIG_EXAMPLE.md) for detailed configuration options and kalo.yaml integration.

## Testing

### Run Integration Tests
```bash
go test ./pkg/compile -v
```

### Validate Generated Code
```bash
python testdata/validate_syntax.py
```

The plugin includes:
- **Ground truth tests** that ensure output matches expected files
- **Syntax validation** to ensure generated Python is valid
- **Example test scripts** showing how to use the generated code

## Development Experience

This plugin was created using the morphe-types-template with these metrics:
- **Time to working plugin**: ~25 minutes
- **DX Score**: 8.5/10
- **Main friction**: Import path updates and Python's import complexity

See [DX-EVAL.md](DX-EVAL.md) for a detailed developer experience report.

## Project Structure

```
plugin-morphe-py-types/
├── cmd/plugin/          # Entry point
├── pkg/
│   ├── compile/         # Core compilation logic
│   ├── formatdef/       # Python type definitions
│   └── typemap/         # Morphe → Python type mappings
├── testdata/            # Test schemas and ground truth
│   ├── registry/        # Input test schemas
│   └── ground-truth/    # Expected outputs
└── output/             # Generated Python code
```

## Known Limitations

- Enum imports in models are tracked but require the enums to be accessible
- Generated code uses relative imports (standard for Python packages)
- Entity relationship loading is stubbed (requires actual implementation)

## License

Same as other Morphe plugins.