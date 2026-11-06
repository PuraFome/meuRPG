package characters

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The owner of an option of a feat is the feat: a player whose sheet keeps the feat reads its options even
// when the master switched the feat off, and one whose sheet does not keep it does not.
func TestAFeatIsTheOwnerOfItsOptionsForAPlayerWhoHoldsIt(t *testing.T) {
	t.Parallel()
	srd, err := rules.LoadSRD()
	if err != nil {
		t.Fatal(err)
	}
	const feat = "feat:resistente@mesa"
	const option = "feature:opcao-a@mesa"
	content, err := srd.With(rules.Overlay{Revision: 1, Off: []string{feat}, Feats: []rules.TableFeat{{
		TableEntry: rules.TableEntry{Key: feat, NamePT: "Resistente"}, DescPT: []string{"x"},
		Effects: []rules.Effect{{Type: "choice", Choice: "feature", Count: 1}},
		Options: []rules.TableFeature{{Key: option, NamePT: "Opção A", DescPT: []string{"A."}}},
	}}})
	if err != nil {
		t.Fatalf("With() error = %v", err)
	}
	l := &tableListing{content: content, mine: map[string]bool{feat: true}}
	if !l.heldByOwner(option) {
		t.Error("a player who holds the feat is denied its option")
	}
	l.mine = map[string]bool{}
	if l.heldByOwner(option) {
		t.Error("a player who does not hold the feat reads its option")
	}
}
