package play

import (
	"context"
	"strings"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// A roll that a feature changed names the feature in Portuguese next to its content key (Reroll.source_name,
// RerollQuestion.source_name and every treated_as_source_name), so the screen can say "Dado Sortudo da Mesa" for a table
// feature, not only for the SRD ones it knows. The name is added to the answer of an RPC, after every rule of who reads
// what has run (RN-10, RN-20): it follows the key and appears only where the key is already there, which is the roller's
// player and the master. An entry the master retired or switched off has no name (RN-23).

// sourceNamePair is a key field of a message and the field that carries its name.
type sourceNamePair struct{ key, name protoreflect.FieldDescriptor }

// sourcePairs are the key and name fields of a message type, none for a type with no pair.
var sourcePairs sync.Map // protoreflect.FullName -> []sourceNamePair

func pairsOf(md protoreflect.MessageDescriptor) []sourceNamePair {
	if v, ok := sourcePairs.Load(md.FullName()); ok {
		return v.([]sourceNamePair)
	}
	var out []sourceNamePair
	fields := md.Fields()
	for i := range fields.Len() {
		k := fields.Get(i)
		if k.Kind() != protoreflect.StringKind || k.IsList() || (k.Name() != "source_key" && !strings.HasSuffix(string(k.Name()), "treated_as_source")) {
			continue
		}
		suffix := "_name"
		if k.Name() == "source_key" {
			suffix = ""
		}
		want := strings.TrimSuffix(string(k.Name()), "_key") + suffix
		if k.Name() == "source_key" {
			want = "source_name"
		}
		if n := fields.ByName(protoreflect.Name(want)); n != nil && n.Kind() == protoreflect.StringKind && !n.IsList() {
			out = append(out, sourceNamePair{key: k, name: n})
		}
	}
	sourcePairs.Store(md.FullName(), out)
	return out
}

// namedSources are the messages of a tree that carry a key to name, with their pairs.
type namedSource struct {
	msg   protoreflect.Message
	pairs []sourceNamePair
}

func collectSources(m protoreflect.Message, out *[]namedSource) {
	if ps := pairsOf(m.Descriptor()); len(ps) > 0 {
		*out = append(*out, namedSource{msg: m, pairs: ps})
	}
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsMap():
			if fd.MapValue().Message() != nil {
				v.Map().Range(func(_ protoreflect.MapKey, mv protoreflect.Value) bool {
					collectSources(mv.Message(), out)
					return true
				})
			}
		case fd.Message() == nil:
		case fd.IsList():
			for i := range v.List().Len() {
				collectSources(v.List().Get(i).Message(), out)
			}
		default:
			collectSources(v.Message(), out)
		}
		return true
	})
}

// nameSources fills the names of the keys a response carries, with the names the campaign's content gives.
func (s *Service) nameSources(ctx context.Context, campaignID string, res proto.Message) {
	var found []namedSource
	collectSources(res.ProtoReflect(), &found)
	var keyed []namedSource
	for _, f := range found {
		for _, p := range f.pairs {
			if f.msg.Get(p.key).String() != "" {
				keyed = append(keyed, f)
				break
			}
		}
	}
	if len(keyed) == 0 || campaignID == "" {
		return
	}
	namer, err := s.roster.SourceNames(ctx, nil, campaignID)
	if err != nil {
		s.logger.WarnContext(ctx, "play: cannot read the content names of the rolls", "error", err)
		return
	}
	for _, f := range keyed {
		for _, p := range f.pairs {
			if key := f.msg.Get(p.key).String(); key != "" {
				f.msg.Set(p.name, protoreflect.ValueOfString(namer(key)))
			}
		}
	}
}

// sourceNameInterceptor names the features of the rolls an answer carries.
func sourceNameInterceptor(s *Service) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			res, err := next(ctx, req)
			if err != nil || res == nil {
				return res, err
			}
			if msg, ok := res.Any().(proto.Message); ok {
				s.nameSources(ctx, campaignOf(req.Any()), msg)
			}
			return res, err
		}
	}
}
