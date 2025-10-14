# Model Consolidation Summary

## Problem
`MockExpectation` was defined in TWO places:
1. `internal/models/config.go` - Generic version with `map[string]interface{}`
2. `internal/builders/common.go` - Rich version with structured fields

This caused confusion and maintainability issues.

## Solution

### Step 1: Create Canonical Model ✅
**File**: `internal/models/expectation.go`
- Moved the BETTER version (from builders) to models
- This is now the SINGLE source of truth for MockExpectation
- Includes all supporting types:
  - `Times`
  - `CallbackConfig`
  - `HttpCallback`
  - `ConnectionOptions`
  - Matching strategies (PathMatchingStrategy, QueryParamMatchingStrategy, RequestBodyMatchingStrategy)

### Step 2: Update Config ✅
**File**: `internal/models/config.go`
- Removed duplicate `MockExpectation` definition
- Removed old `ExpectationTimes` (now using `Times` from expectation.go)
- Kept only config-specific types:
  - `ConfigMetadata`
  - `MockConfiguration`
  - `ConfigSettings`
  - `TimeToLive` (different from Times!)
  - `Delay`
  - `VersionInfo`
  - `ProjectInfo`
  - `ValidationError`

### Step 3: Extract Validation Utilities ✅
**File**: `internal/models/validation.go`
- Moved JSON/regex validation functions here
- `ValidateJSON()` - Uses JSONValidationError
- `FormatJSON()` - Uses JSONValidationError
- `IsValidRegex()` - Uses RegexValidationError

### Step 4: Update Builders Package ✅
**File**: `internal/builders/common.go`
- Removed duplicate type definitions
- Now re-exports types from models for backward compatibility
- Re-exports functions for backward compatibility
- No breaking changes to existing code!

## File Structure

```
internal/models/
├── expectation.go      # MockExpectation + conversion to MockServer JSON
├── config.go           # Configuration metadata and project info
├── validation.go       # JSON/regex validation utilities
└── errors.go           # Custom error types (already existed)

internal/builders/
├── common.go          # Re-exports from models + helper functions
├── rest.go            # REST expectation builder
├── graphql.go         # GraphQL expectation builder
└── mock_configurator.go  # Mock configuration helpers
```

## Benefits

1. **Single Source of Truth**: MockExpectation defined in ONE place
2. **Better Organization**: Models in models/, builders use models
3. **No Breaking Changes**: Re-exports maintain backward compatibility
4. **Clearer Separation**: 
   - Models = Data structures
   - Builders = Interactive building logic
5. **Easier Maintenance**: Change MockExpectation in one place

## What's Next

Now that models are consolidated, we can safely proceed with:
- #3 Configuration Management
- #4 Structured Logging  
- #5 Context Propagation

All future changes will reference the canonical `models.MockExpectation`!
