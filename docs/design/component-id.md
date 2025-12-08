# Component ID Design

## Overview

Component IDs are generated deterministically using a hash-based approach to ensure stability across multiple scans of the same codebase.

## Algorithm

The ID is generated using SHA-256 hash of the component's name and relative path:

```
content = "name:relativePath"
hash = SHA256(content)
component_id = hex_encode(hash)[:20]
```

## Implementation

- **Function**: `GenerateID(name, relativePath string) string` in `internal/types/nanoid.go`
- **Length**: 20 hexadecimal characters
- **Collision Probability**: Extremely low (2^80 possibilities)
- **Path Type**: Relative path for portability across different environments

## Benefits

- **Stable IDs**: Same component generates identical ID across scans
- **Portable**: Works regardless of absolute file system location
- **Unique**: Sufficient for practical use with collision probability
- **Extensible**: Future enhancements can incorporate additional factors

## Usage

Called from `NewPayload()` in `internal/types/payload.go` using the component name and first path from the paths array.
