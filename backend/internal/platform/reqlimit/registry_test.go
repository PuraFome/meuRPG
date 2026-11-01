package reqlimit

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	_ "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	_ "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	_ "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	_ "github.com/PuraFome/meuRPG/backend/gen/meurpg/identity/v1"
	_ "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	_ "github.com/PuraFome/meuRPG/backend/gen/meurpg/notes/v1"
	_ "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	_ "github.com/PuraFome/meuRPG/backend/gen/meurpg/progression/v1"
	_ "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	_ "github.com/PuraFome/meuRPG/backend/gen/meurpg/system/v1"
)

// procedure is one RPC of the API.
type procedure struct {
	name  string // "/meurpg.x.v1.Service/Method"
	input protoreflect.MessageDescriptor
}

func procedures(t *testing.T) []procedure {
	t.Helper()
	var out []procedure
	protoregistry.GlobalFiles.RangeFiles(func(f protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(string(f.Package()), "meurpg.") {
			return true
		}
		for i := range f.Services().Len() {
			svc := f.Services().Get(i)
			for j := range svc.Methods().Len() {
				m := svc.Methods().Get(j)
				out = append(out, procedure{"/" + string(svc.FullName()) + "/" + string(m.Name()), m.Input()})
			}
		}
		return true
	})
	if len(out) < 100 {
		t.Fatalf("found only %d procedures: the services are not registered", len(out))
	}
	return out
}

// reachableFields calls visit for every field of md and of the messages it
// reaches (message fields, list elements, map values), each message once.
// It follows descriptors, so the visited set is what ends a recursive type.
func reachableFields(md protoreflect.MessageDescriptor, visit func(protoreflect.FieldDescriptor)) {
	seen := map[protoreflect.FullName]bool{}
	var walk func(protoreflect.MessageDescriptor)
	walk = func(md protoreflect.MessageDescriptor) {
		if seen[md.FullName()] {
			return
		}
		seen[md.FullName()] = true
		for i := range md.Fields().Len() {
			fd := md.Fields().Get(i)
			visit(fd)
			switch {
			case fd.IsMap():
				if v := fd.MapValue(); v.Message() != nil {
					walk(v.Message())
				}
			case fd.Message() != nil:
				walk(fd.Message())
			}
		}
	}
	walk(md)
}

// TestRegistry checks the whole API against the limits, with no database.
//
//   - Every procedure has a read limit that is at least the default and at
//     most the backstop, and every entry of raisedReadLimits names a real
//     procedure and raises the default.
//   - Every field a request reaches that is a list, a map, a string or bytes
//     has a limit that is not 0 (that is the guarantee: no field is unbounded).
//   - A procedure that reaches a field whose option raises its limit over the
//     default has a raised read limit. The test does not add up a worst case,
//     which would be a number nobody can keep right by hand: it makes the
//     author of the option decide the procedure's read limit on purpose.
func TestRegistry(t *testing.T) {
	procs := procedures(t)
	known := map[string]bool{}
	for _, p := range procs {
		known[p.name] = true
		if n := ReadLimit(p.name); n < DefaultReadLimit || n > Backstop {
			t.Errorf("%s: read limit %d is outside %d..%d", p.name, n, DefaultReadLimit, Backstop)
		}
		reachableFields(p.input, func(fd protoreflect.FieldDescriptor) {
			checkBounded(t, p.name, fd)
			if _, raised := raisedReadLimits[p.name]; Raised(fd) && !raised {
				t.Errorf("%s reaches %s, which raises its limit over the defaults, but the procedure has no entry in raisedReadLimits", p.name, fd.FullName())
			}
		})
	}
	for name, n := range raisedReadLimits {
		switch {
		case !known[name]:
			t.Errorf("raisedReadLimits names %s, which is not a procedure", name)
		case n <= DefaultReadLimit:
			t.Errorf("raisedReadLimits[%s] = %d does not raise the default %d", name, n, DefaultReadLimit)
		case n > Backstop:
			t.Errorf("raisedReadLimits[%s] = %d is over the backstop %d", name, n, Backstop)
		}
	}
	// The test above can only pass for the right reason if the walk sees what a request holds.
	var stringFields int
	for _, p := range procs {
		reachableFields(p.input, func(fd protoreflect.FieldDescriptor) {
			if fd.Kind() == protoreflect.StringKind {
				stringFields++
			}
		})
	}
	if stringFields < 100 {
		t.Errorf("the walk found only %d string fields", stringFields)
	}
}

// checkBounded fails when a field that can hold an unbounded amount has no limit.
func checkBounded(t *testing.T, procedure string, fd protoreflect.FieldDescriptor) {
	t.Helper()
	kinds := []protoreflect.Kind{fd.Kind()}
	if fd.IsMap() {
		kinds = []protoreflect.Kind{fd.MapKey().Kind(), fd.MapValue().Kind()}
	}
	if (fd.IsList() || fd.IsMap()) && (ItemsLimit(fd) == 0 || EffectiveLimit(fd) == 0) {
		t.Errorf("%s: %s has no item limit", procedure, fd.FullName())
	}
	for _, k := range kinds {
		isText := k == protoreflect.StringKind || k == protoreflect.BytesKind
		if isText && LenLimit(fd, k) == 0 {
			t.Errorf("%s: %s has no length limit", procedure, fd.FullName())
		}
	}
	if !fd.IsList() && !fd.IsMap() && (fd.Kind() == protoreflect.StringKind || fd.Kind() == protoreflect.BytesKind) && EffectiveLimit(fd) == 0 {
		t.Errorf("%s: %s has no limit", procedure, fd.FullName())
	}
}
