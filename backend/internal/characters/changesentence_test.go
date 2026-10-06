package characters

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// TestChangeSentenceNamesOnlyTheChangedEntry: "A classe mudou" names the class or
// subclass a sentence is about only when it is the entry that changed; for any
// other it tells the same sentence without a name; an issue with no change
// sentence keeps its own (review of slice 10.3, the SRD Ladino of a table race).
func TestChangeSentenceNamesOnlyTheChangedEntry(t *testing.T) {
	t.Parallel()
	is := rules.Issue{Message: "Há 5 perícias escolhidas; o personagem escolhe 4.", ChangeMessage: "agora dá 4 perícias no nível 1; esta ficha tem 5.", ChangeSubject: "class:rogue"}
	for _, tc := range []struct {
		name, changed, want string
	}{
		{"the subject changed", "class:rogue", "Ladino agora dá 4 perícias no nível 1; esta ficha tem 5."},
		{"another entry changed", "race:perito@mesa", "Agora dá 4 perícias no nível 1; esta ficha tem 5."},
	} {
		if got := changeSentence(is, tc.changed, "Ladino"); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
	plain := rules.Issue{Message: "Raio não está na lista."}
	if got := changeSentence(plain, "spell:raio@mesa", "Raio"); got != plain.Message {
		t.Errorf("an issue with no change sentence = %q, want its own", got)
	}
	neutral := rules.Issue{Message: "m", ChangeMessage: "o total de perícias para escolher agora é 1; esta ficha tem 2."}
	if got := changeSentence(neutral, "race:x@mesa", "X"); got != "O total de perícias para escolher agora é 1; esta ficha tem 2." {
		t.Errorf("neutral sentence = %q", got)
	}
}
