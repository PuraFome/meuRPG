package leaktest

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// inspect looks for what p must never read in one answer and returns what it
// found, one line each. Three layers, from the crudest to the most precise:
//
//  1. the raw body, as the app receives it: no marker and no hidden id of a
//     canary p may not read (this covers error messages and every field, known or not);
//  2. the decoded message: no secret number in a numeric field (a trap DC of 29
//     is searched as a number, never as text);
//  3. the master-only fields (masterOnly, below), which a player must read empty
//     whatever their value, so a field renamed or added to the wrong message
//     is caught even when its content is something the canaries never held.
//
// ignore lists the canary kinds this answer may hold legitimately (public SRD data).
func (w *world) inspect(p *person, r reply, ignore []string) []string {
	var found []string
	for _, c := range w.secrets.list {
		if c.mayRead(p) || slices.Contains(ignore, c.kind) || c.numeric {
			continue
		}
		if bytes.Contains(r.body, []byte(c.needle)) {
			found = append(found, "the answer holds "+c.String())
		}
	}
	if r.msg == nil {
		return found
	}
	var numbers []*canary
	for _, c := range w.secrets.list {
		if c.numeric && !c.mayRead(p) && !slices.Contains(ignore, c.kind) {
			numbers = append(numbers, c)
		}
	}
	walk(r.msg.ProtoReflect(), r.msg.ProtoReflect().Descriptor().Name(), func(path string, m protoreflect.Message, fd protoreflect.FieldDescriptor, v protoreflect.Value) {
		if isNumber(fd) {
			for _, c := range numbers {
				if c.matchesNumber(fd, asInt(fd, v)) {
					found = append(found, fmt.Sprintf("%s holds the secret number %s", path, c))
				}
			}
		}
		for _, rule := range masterOnly {
			if rule.matches(m, fd) && rule.leaks(w, p, m) {
				found = append(found, fmt.Sprintf("%s is a master-only field (%s)", path, rule.why))
			}
		}
	})
	return found
}

// matchesNumber says whether a numeric field holds the secret number.
func (c *canary) matchesNumber(fd protoreflect.FieldDescriptor, n int64) bool {
	if n != c.value {
		return false
	}
	if len(c.fields) == 0 {
		return true
	}
	return slices.ContainsFunc(c.fields, func(word string) bool { return strings.Contains(string(fd.Name()), word) })
}

// walk visits every populated field of m, in lists and maps and nested messages.
func walk(m protoreflect.Message, path protoreflect.Name, visit func(path string, m protoreflect.Message, fd protoreflect.FieldDescriptor, v protoreflect.Value)) {
	walkAt(m, string(path), visit)
}

func walkAt(m protoreflect.Message, path string, visit func(path string, m protoreflect.Message, fd protoreflect.FieldDescriptor, v protoreflect.Value)) {
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		here := path + "." + string(fd.Name())
		switch {
		case fd.IsList():
			l := v.List()
			for i := range l.Len() {
				at := fmt.Sprintf("%s[%d]", here, i)
				if fd.Message() != nil {
					visit(at, m, fd, protoreflect.ValueOfMessage(l.Get(i).Message()))
					walkAt(l.Get(i).Message(), at, visit)
				} else {
					visit(at, m, fd, l.Get(i))
				}
			}
		case fd.IsMap():
			v.Map().Range(func(k protoreflect.MapKey, mv protoreflect.Value) bool {
				at := fmt.Sprintf("%s[%v]", here, k)
				if fd.MapValue().Message() != nil {
					walkAt(mv.Message(), at, visit)
				} else {
					visit(at, m, fd.MapValue(), mv)
				}
				return true
			})
		case fd.Message() != nil:
			visit(here, m, fd, v)
			walkAt(v.Message(), here, visit)
		default:
			visit(here, m, fd, v)
		}
		return true
	})
}

func isNumber(fd protoreflect.FieldDescriptor) bool {
	switch fd.Kind() {
	case protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.Sint32Kind, protoreflect.Sint64Kind, protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
		return true
	}
	return false
}

func asInt(fd protoreflect.FieldDescriptor, v protoreflect.Value) int64 {
	if !isNumber(fd) || !v.IsValid() {
		return -1
	}
	switch fd.Kind() {
	case protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
		return int64(v.Uint()) //nolint:gosec // G115: compared with a small constant
	}
	return v.Int()
}

// masterOnlyField is a field that a player always reads empty. A rule says which
// message and field, why, and when a player may read it after all (unless): a
// player's own character, for instance. Every rule is checked against the
// descriptors by TestMasterOnlyFieldsExist, so a rename breaks the test.
type masterOnlyField struct {
	message protoreflect.FullName
	field   protoreflect.Name
	why     string
	// unless, when set, says whether p may read the field in this message.
	unless func(w *world, p *person, m protoreflect.Message) bool
}

func (r masterOnlyField) matches(m protoreflect.Message, fd protoreflect.FieldDescriptor) bool {
	return m.Descriptor().FullName() == r.message && fd.Name() == r.field
}

func (r masterOnlyField) leaks(w *world, p *person, m protoreflect.Message) bool {
	if r.unless != nil && r.unless(w, p, m) {
		return false
	}
	return true
}

var (
	_ = proto.Marshal
	_ = strings.Contains
)
