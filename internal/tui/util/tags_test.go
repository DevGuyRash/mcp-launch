package util

import (
    "testing"

    "mcp-launch/internal/tui/state"
)

func findKind(tags []state.Tag, k state.TagKind) (idx int, ok bool) {
    for i, t := range tags {
        if t.Kind == k {
            return i, true
        }
    }
    return -1, false
}

func TestTrimmedVsTruncatedExclusivity(t *testing.T) {
    raw := "Hello world this is a sentence"
    limit := 8 // word-safe trim should produce "Hello"
    trimmed := "Hello"
    truncated := raw[:limit]

    // Trimmed case
    tags := ComputeTags(raw, trimmed, limit, false)
    if _, ok := findKind(tags, state.TRIMMED); !ok {
        t.Fatalf("expected TRIMMED tag present")
    }
    if _, ok := findKind(tags, state.TRUNCATED); ok {
        t.Fatalf("did not expect TRUNCATED tag when TRIMMED applies")
    }

    // Truncated case
    tags = ComputeTags(raw, truncated, limit, false)
    if _, ok := findKind(tags, state.TRUNCATED); !ok {
        t.Fatalf("expected TRUNCATED tag present")
    }
    if _, ok := findKind(tags, state.TRIMMED); ok {
        t.Fatalf("did not expect TRIMMED tag when TRUNCATED applies")
    }
}

func TestOverLimitMathAndCounters(t *testing.T) {
    raw := "1234567890"
    limit := 7
    mod := raw // assume no modification for counter checks

    tags := ComputeTags(raw, mod, limit, false)

    if idx, ok := findKind(tags, state.OVER_LIMIT); !ok {
        t.Fatalf("expected OVER_LIMIT tag present")
    } else {
        if tags[idx].Value != len([]rune(raw))-limit {
            t.Fatalf("unexpected over-limit value: got %d", tags[idx].Value)
        }
    }

    if idx, ok := findKind(tags, state.ORIG_LEN); !ok || tags[idx].Value != len([]rune(raw)) {
        t.Fatalf("expected ORIG_LEN with correct value")
    }
    if idx, ok := findKind(tags, state.MOD_LEN); !ok || tags[idx].Value != len([]rune(mod)) {
        t.Fatalf("expected MOD_LEN with correct value")
    }
}

func TestStableOrder(t *testing.T) {
    raw := "Hello world"
    limit := 8
    trimmed := "Hello"
    tags := ComputeTags(raw, trimmed, limit, true /*edited*/)
    // Expected order: EDITED, TRIMMED, OVER_LIMIT, ORIG_LEN, MOD_LEN
    order := []state.TagKind{state.EDITED, state.TRIMMED, state.OVER_LIMIT, state.ORIG_LEN, state.MOD_LEN}
    pos := map[state.TagKind]int{}
    for i, tg := range tags {
        pos[tg.Kind] = i
    }
    prev := -1
    for _, k := range order {
        if idx, ok := pos[k]; ok {
            if idx < prev {
                t.Fatalf("tag %v appears before previous; order unstable", k)
            }
            prev = idx
        }
    }
}

