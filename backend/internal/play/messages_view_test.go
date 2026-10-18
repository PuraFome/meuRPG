package play

import (
	"testing"
	"time"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Only an erased account is a removed author: a present author who never chose a display name has no
// name to show, which is not the same thing (the proto says author_removed means the account was erased).
func TestOutOfCharacterAuthorRemovedMeansAnErasedAccount(t *testing.T) {
	author := "u-1"
	row := func(author *string) playdb.SessionMessage {
		return playdb.SessionMessage{ID: "m1", SpeakerKind: speakerOutOfCharacter, AuthorUserID: author, Text: "oi", ForEveryone: true, CreatedAt: time.Unix(0, 0)}
	}
	cases := []struct {
		name        string
		author      *string
		names       map[string]string
		wantName    string
		wantRemoved bool
	}{
		{"a present author with a name", &author, map[string]string{"u-1": "Celso"}, "Celso", false},
		{"a present author who never chose a name", &author, map[string]string{}, "", false},
		{"an erased account", nil, map[string]string{"u-1": "Celso"}, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := messageToProto(row(c.author), nil, nil, c.names, false)
			if got.GetSpeakerName() != c.wantName || got.GetAuthorRemoved() != c.wantRemoved {
				t.Errorf("speaker_name = %q, author_removed = %v; want %q and %v", got.GetSpeakerName(), got.GetAuthorRemoved(), c.wantName, c.wantRemoved)
			}
		})
	}
}
