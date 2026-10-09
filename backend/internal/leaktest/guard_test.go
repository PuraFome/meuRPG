package leaktest

import (
	"slices"
	"sort"
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	// Every service's generated descriptors register themselves on import: the guard
	// walks the registry, so a service whose package is not imported is not seen.
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

// allProcedures lists every procedure of every meurpg service, as the generated
// descriptors say: "/meurpg.maps.v1.MapService/GetMap", and whether it streams.
func allProcedures() map[string]bool {
	out := map[string]bool{}
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(string(fd.Package()), "meurpg.") {
			return true
		}
		for i := range fd.Services().Len() {
			svc := fd.Services().Get(i)
			for j := range svc.Methods().Len() {
				m := svc.Methods().Get(j)
				out["/"+string(svc.FullName())+"/"+string(m.Name())] = m.IsStreamingServer()
			}
		}
		return true
	})
	return out
}

// TestEveryProcedureIsClassified is the completeness guard: it walks the generated service
// descriptors and fails for a procedure that is in none of the tables (a new RPC must say
// whether it is a read, and then which rows ask it, or what else it is), for one that is in
// two, and for a table row that names a procedure that does not exist any more.
func TestEveryProcedureIsClassified(t *testing.T) {
	all := allProcedures()
	where := map[string][]string{}
	for _, r := range reads {
		where[r.procedure] = appendOnce(where[r.procedure], "reads")
	}
	for _, r := range readsAfterTheCombat {
		where[r.procedure] = appendOnce(where[r.procedure], "readsAfterTheCombat")
	}
	for _, r := range readsAfterTheSession {
		where[r.procedure] = appendOnce(where[r.procedure], "readsAfterTheSession")
	}
	for _, r := range actions {
		where[r.procedure] = appendOnce(where[r.procedure], "actions")
	}
	for p, c := range notReads {
		where[p] = appendOnce(where[p], "notReads")
		if strings.TrimSpace(c.why) == "" {
			t.Errorf("%s is in notReads with no reason", p)
		}
	}
	var missing []string
	for p, streaming := range all {
		switch places := where[p]; {
		case len(places) == 0:
			missing = append(missing, strings.TrimPrefix(p, "/meurpg."))
		case len(places) > 1:
			t.Errorf("%s is in more than one table: %v", p, places)
		case streaming && places[0] != "notReads":
			t.Errorf("%s streams: the matrix calls unary procedures only; classify it as streamed and test it in stream_test.go", p)
		}
	}
	sort.Strings(missing)
	for _, p := range missing {
		t.Errorf("%s is not classified: add a row to 'reads' (reads_test.go) if it returns data, or a line to 'notReads' (classify_test.go) with its class and the reason", p)
	}
	for p := range where {
		if _, ok := all[p]; !ok {
			t.Errorf("%s is in a table but is not a procedure of any service any more: remove it", p)
		}
	}
}

func appendOnce(s []string, v string) []string {
	if slices.Contains(s, v) {
		return s
	}
	return append(s, v)
}
