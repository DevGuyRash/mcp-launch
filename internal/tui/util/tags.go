package util

import (
    "strings"
    "unicode"

    "mcp-launch/internal/tui/state"
)

// ComputeTags calculates the set of status tags for a description given the
// raw (original) text, the modified (current) text, the character limit, and
// whether the user has made manual edits.
//
// The returned slice preserves a stable order:
//   Edited, Trimmed, Truncated, Over Limit, Orig Len, Mod Len
//
// Rules:
// - Edited reflects explicit user edits (independent of auto-shortening).
// - Trimmed and Truncated are mutually exclusive heuristics based on comparing
//   modified against word-safe trim and hard truncation of raw.
// - Over Limit is computed from the raw length (raw_len - limit when positive).
// - Orig Len and Mod Len are always included (counters).
func ComputeTags(raw, modified string, limit int, edited bool) []state.Tag {
    rawLen := runeLen(raw)
    modLen := runeLen(modified)

    // Expected automatic transformation results derived from raw
    wsTrim := wordSafeTrim(raw, limit)
    hardCut := hardTruncate(raw, limit)

    // Determine which shortening (if any) matches the modified text.
    // We compare exact strings to avoid guesswork; this yields deterministic tags.
    trimmed := (rawLen > modLen) && (modified == wsTrim) && (wsTrim != hardCut)
    truncated := (rawLen > modLen) && (modified == hardCut)

    // Enforce mutual exclusivity: if both would be true (edge case where
    // wsTrim equals hardCut), prefer Trimmed semantics only if wsTrim differs
    // from hardCut (handled in the 'trimmed' predicate above). Otherwise, only
    // one can be true at a time.

    over := 0
    if limit > 0 && rawLen > limit {
        over = rawLen - limit
    }

    tags := make([]state.Tag, 0, 6)

    // 1) Edited
    if edited {
        tags = append(tags, state.Tag{Kind: state.EDITED})
    }

    // 2) Trimmed
    if trimmed {
        tags = append(tags, state.Tag{Kind: state.TRIMMED})
    }

    // 3) Truncated
    if !trimmed && truncated { // ensure exclusivity
        tags = append(tags, state.Tag{Kind: state.TRUNCATED})
    }

    // 4) Over Limit (+N)
    if over > 0 {
        tags = append(tags, state.Tag{Kind: state.OVER_LIMIT, Value: over})
    }

    // 5) Original Length (N)
    tags = append(tags, state.Tag{Kind: state.ORIG_LEN, Value: rawLen})

    // 6) Modified Length (M)
    tags = append(tags, state.Tag{Kind: state.MOD_LEN, Value: modLen})

    return tags
}

// wordSafeTrim returns a version of s that does not exceed limit runes by
// cutting at the last whitespace boundary before limit. If no boundary exists,
// falls back to hard truncation at the limit. Leading/trailing whitespace from
// the cut result is removed.
func wordSafeTrim(s string, limit int) string {
    r := []rune(s)
    if limit <= 0 || len(r) <= limit {
        return s
    }
    cut := -1
    boundary := -1
    // Track last whitespace index before the limit
    for i := 0; i < limit && i < len(r); i++ {
        if unicode.IsSpace(r[i]) {
            boundary = i
        }
        cut = i
    }
    if boundary >= 0 {
        return strings.TrimSpace(string(r[:boundary]))
    }
    return string(r[:cut+1])
}

// hardTruncate returns s cut to at most limit runes.
func hardTruncate(s string, limit int) string {
    r := []rune(s)
    if limit <= 0 || len(r) <= limit {
        return s
    }
    return string(r[:limit])
}

// runeLen returns the length of s in runes (Unicode code points).
func runeLen(s string) int {
    return len([]rune(s))
}
