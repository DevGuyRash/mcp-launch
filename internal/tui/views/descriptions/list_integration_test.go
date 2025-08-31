package descriptions

import (
    "strings"
    "testing"

    utiltags "mcp-launch/internal/tui/util"
)

func TestRenderTagsIntegration(t *testing.T) {
    raw := "Hello world this is a sentence"
    limit := 8
    mod := "Hello"
    tags := utiltags.ComputeTags(raw, mod, limit, true) // edited + trimmed
    out := RenderTags(tags, true) // noColor

    wants := []string{"[Edited]", "[Trimmed]", "[Over +", "[Orig ", "[Mod "}
    for _, w := range wants {
        if !strings.Contains(out, w) {
            t.Fatalf("expected %q in output: %s", w, out)
        }
    }
}
