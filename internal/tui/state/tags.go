package state

// TagKind enumerates the types of status tags for descriptions.
type TagKind int

const (
    // Stable ordering for display: Edited, Trimmed, Truncated, Over Limit, Orig Len, Mod Len
    EDITED TagKind = iota
    TRIMMED
    TRUNCATED
    OVER_LIMIT
    ORIG_LEN
    MOD_LEN
)

// Tag represents a single status chip. Value is used for numeric counters
// (e.g., over-limit amount or lengths). Non-numeric tags use Value = 0.
type Tag struct {
    Kind  TagKind
    Value int
}

