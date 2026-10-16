package characters

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// The helpers of the tests that state what a call changed or what a viewer reads: a message
// as flat path=value pairs, and the pairs that differ between two of them.

// protoFlat is a message as a map from the path of each field in its protojson form
// (spellSlots[1].used) to the value as JSON; the times are left out, they come from the
// test's clock.
func protoFlat(t *testing.T, m proto.Message) map[string]string {
	t.Helper()
	raw, err := protojson.Marshal(m)
	if err != nil {
		t.Fatalf("protojson.Marshal() error = %v", err)
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	out := map[string]string{}
	flattenJSON(out, "", tree)
	return out
}

func flattenJSON(out map[string]string, path string, v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			if k == "createdAt" || k == "updatedAt" || k == "archivedAt" {
				continue
			}
			flattenJSON(out, strings.TrimPrefix(path+"."+k, "."), e)
		}
	case []any:
		for i, e := range x {
			flattenJSON(out, fmt.Sprintf("%s[%d]", path, i), e)
		}
	default:
		b, _ := json.Marshal(x)
		out[path] = string(b)
	}
}

// fieldsMoved lists, sorted, the fields of after that differ from before as
// path=value, and the ones it no longer has as -path.
func fieldsMoved(before, after map[string]string) string {
	var moved []string
	for k, v := range after {
		if before[k] != v {
			moved = append(moved, k+"="+v)
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			moved = append(moved, "-"+k)
		}
	}
	slices.Sort(moved)
	if len(moved) == 0 {
		return "unchanged"
	}
	return strings.Join(moved, " ")
}
