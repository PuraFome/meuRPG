package characters

import (
	"encoding/json"
	"fmt"
	"regexp"
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
	return protoFlatWith(t, m, false)
}

// protoFlatKeyed is protoFlat with the elements of a list named by their key or class key
// (classChoices[class:rogue].available) instead of their place, so a list that lost an
// element does not shift the paths of the others.
func protoFlatKeyed(t *testing.T, m proto.Message) map[string]string {
	t.Helper()
	return protoFlatWith(t, m, true)
}

func protoFlatWith(t *testing.T, m proto.Message, keyed bool) map[string]string {
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
	flattenJSON(out, "", tree, keyed)
	return out
}

func flattenJSON(out map[string]string, path string, v any, keyed bool) {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			if k == "createdAt" || k == "updatedAt" || k == "archivedAt" {
				continue
			}
			flattenJSON(out, strings.TrimPrefix(path+"."+k, "."), e, keyed)
		}
	case []any:
		for i, e := range x {
			flattenJSON(out, fmt.Sprintf("%s[%s]", path, elementID(e, i, keyed)), e, keyed)
		}
	default:
		b, _ := json.Marshal(x)
		out[path] = string(b)
	}
}

// elementID names the element i of a list: its class key or key when keyed and it has one,
// else its place.
func elementID(e any, i int, keyed bool) string {
	if obj, ok := e.(map[string]any); ok && keyed {
		for _, k := range []string{"classKey", "key"} {
			if id, ok := obj[k].(string); ok && id != "" {
				return id
			}
		}
	}
	return fmt.Sprint(i)
}

// wholeElements are the keyed elements ("list[class:rogue]") that have fields in a and none
// in b, the outermost ones.
func wholeElements(a, b map[string]string) []string {
	seen := map[string]bool{}
	for path := range a {
		for _, loc := range keyedBracket.FindAllStringIndex(path, -1) {
			seen[path[:loc[1]]] = true
		}
	}
	var cands []string
	for p := range seen {
		if !hasPrefixIn(b, p) {
			cands = append(cands, p)
		}
	}
	slices.SortFunc(cands, func(x, y string) int { return len(x) - len(y) })
	var outer []string
	for _, p := range cands {
		if !underAny(p, outer) {
			outer = append(outer, p)
		}
	}
	return outer
}

// keyedBracket is a list index that is a key, not a number.
var keyedBracket = regexp.MustCompile(`\[[^\]0-9][^\]]*\]`)

func hasPrefixIn(m map[string]string, prefix string) bool {
	for k := range m {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	return false
}

func underAny(path string, prefixes []string) bool {
	return slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(path, p) })
}

// fieldsMoved lists, sorted, the fields of after that differ from before as
// path=value, and the ones it no longer has as -path; a keyed list element that is
// gone or new as a whole is -path or +path of the element.
func fieldsMoved(before, after map[string]string) string {
	gone, added := wholeElements(before, after), wholeElements(after, before)
	var moved []string
	for k, v := range after {
		if before[k] != v && !underAny(k, added) {
			moved = append(moved, k+"="+v)
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok && !underAny(k, gone) {
			moved = append(moved, "-"+k)
		}
	}
	for _, p := range gone {
		moved = append(moved, "-"+p)
	}
	for _, p := range added {
		moved = append(moved, "+"+p)
	}
	slices.Sort(moved)
	if len(moved) == 0 {
		return "unchanged"
	}
	return strings.Join(moved, " ")
}
