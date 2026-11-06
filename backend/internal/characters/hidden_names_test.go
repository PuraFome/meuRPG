package characters

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// RN-23: nothing a player receives names a hidden entry (switched off or retired), in a key or in a text.
// The master reads the names, as the positive control.

const (
	hiddenRaceName   = "Anão das Brumas"
	hiddenTraitName  = "Pele de Bruma"
	hiddenFeatName   = "Dom Oculto de Névoa"
	visibleRaceName  = "Elfo Escarlate"
	hiddenRaceStem   = "anao-das-brumas"
	hiddenTraitStem  = "pele-de-bruma"
	hiddenFeatStem   = "dom-oculto-de-nevoa"
	onlyHiddenName   = "Linhagem Antiga"
	mixedName        = "Linhagem Mista"
	lacksTraitName   = "Sem a pele"
	lacksHiddenFeat  = "Sem o dom"
	openFeatName     = "Aberto ao vale"
	grantingFeatName = "Escolha do vale"
)

// hiddenNamesTable is a campaign where the master switched a race and a feat off, and registered feats that
// name them in their prerequisites.
type hiddenNamesTable struct {
	*levelUpTable
	race, feat, trait, onlyHidden, mixed, lacksTrait, lacksFeat, open string
	background                                                        string
}

func newHiddenNamesTable(t *testing.T) *hiddenNamesTable {
	t.Helper()
	tb := &hiddenNamesTable{levelUpTable: newLevelUpTable(t, 2700)}
	m := tb.master
	setRules(t, m, tb.campaign, func(r *campaignsv1.TableRules) { r.FeatsAllowed = true })
	race := testRace(hiddenRaceName)
	race.Traits[0].NamePt = hiddenTraitName
	raceEntry := m.addEntry(t, tb.campaign, race)
	tb.race, tb.trait = raceEntry.GetKey(), raceEntry.GetTableRace().GetTraits()[0].GetKey()
	scarlet := m.addEntry(t, tb.campaign, testRace(visibleRaceName)).GetKey()
	tb.feat = m.addEntry(t, tb.campaign, &rulesv1.TableFeat{NamePt: hiddenFeatName, DescPt: []string{"Escondido."}}).GetKey()
	feat := func(name string, p *rulesv1.FeatPrerequisite) string {
		return m.addEntry(t, tb.campaign, &rulesv1.TableFeat{NamePt: name, DescPt: []string{"Texto."}, Prerequisite: p}).GetKey()
	}
	tb.onlyHidden = feat(onlyHiddenName, &rulesv1.FeatPrerequisite{Races: []string{tb.race}})
	tb.mixed = feat(mixedName, &rulesv1.FeatPrerequisite{Races: []string{tb.race, scarlet}, LacksFeatureKey: tb.feat})
	tb.lacksTrait = feat(lacksTraitName, &rulesv1.FeatPrerequisite{LacksFeatureKey: tb.trait})
	tb.lacksFeat = feat(lacksHiddenFeat, &rulesv1.FeatPrerequisite{LacksFeatureKey: tb.feat})
	tb.open = feat(openFeatName, nil)
	// A background whose feature lets the player pick a feat when the sheet is made.
	bg := testBackground("Herdeiro do vale")
	bg.Feature = &rulesv1.TableFeature{
		NamePt: grantingFeatName, DescPt: []string{"Um talento à escolha."},
		Effects: []*rulesv1.TableEffect{{Type: "choice", Choice: "feat", Count: 1}},
	}
	tb.background = m.addEntry(t, tb.campaign, bg).GetKey()
	m.setOff(t, tb.campaign, true, tb.race, tb.feat)
	return tb
}

// hiddenWords are what no player payload may carry: the keys' stems and the Portuguese names.
var hiddenWords = []string{
	hiddenRaceName, hiddenTraitName, hiddenFeatName, hiddenRaceStem, hiddenTraitStem, hiddenFeatStem,
}

// wantNames fails for each word the message does not carry (present) or carries (not present), searched in its prototext.
func wantNames(t *testing.T, what string, m proto.Message, present bool, words ...string) {
	t.Helper()
	text := prototext.MarshalOptions{Multiline: true}.Format(m)
	for _, w := range words {
		if has := strings.Contains(text, w); has != present {
			t.Errorf("%s: carries %q = %v, want %v\n%s", what, w, has, present, text)
		}
	}
}

func TestAPlayersLevelUpOptionsNameNoHiddenEntry(t *testing.T) {
	t.Parallel()
	tb := newHiddenNamesTable(t)
	player, err := tb.options(tb.owner, tb.pc)
	if err != nil {
		t.Fatal(err)
	}
	master, err := tb.options(tb.master, tb.pc)
	if err != nil {
		t.Fatal(err)
	}
	// The positive control: the master reads the hidden race, trait and feat in the feats that name them.
	wantNames(t, "the master's options", master, true, hiddenFeatName, hiddenTraitName, tb.race, tb.feat, tb.trait)
	wantNames(t, "the player's options", player, false, hiddenWords...)
	wantNames(t, "the player's options", player, false, tb.race, tb.feat, tb.trait)
	// The player is offered the feats they can read, and the one with a visible race says only that race.
	if optionOf(player, tb.open) == nil || optionOf(player, tb.lacksTrait) == nil || optionOf(player, tb.mixed) == nil {
		t.Errorf("the player's feats lack a live one: %v", player.GetFeats())
	}
	// A feat that asks only for a hidden race is not offered to them (their prerequisite would read as none), nor hinted at.
	if optionOf(player, tb.onlyHidden) != nil || optionOf(master, tb.onlyHidden) == nil {
		t.Errorf("the feat of the hidden race: player %v, master %v; want the master's only", optionOf(player, tb.onlyHidden), optionOf(master, tb.onlyHidden))
	}
	wantNames(t, "the player's options", player, false, onlyHiddenName, tb.onlyHidden)
	if got := optionOf(player, tb.mixed).GetPrerequisite().GetRaces(); len(got) != 1 {
		t.Errorf("the mixed feat's races for the player = %v, want only the visible one", got)
	}
}

func TestAPlayersCopyOfAFeatNamesNoHiddenEntry(t *testing.T) {
	t.Parallel()
	tb := newHiddenNamesTable(t)
	list := func(u *user) *rulesv1.ListTableEntriesResponse {
		res, err := u.table.ListTableEntries(t.Context(), connect.NewRequest(&rulesv1.ListTableEntriesRequest{CampaignId: tb.campaign}))
		if err != nil {
			t.Fatal(err)
		}
		return res.Msg
	}
	master, player := list(tb.master), list(tb.owner)
	wantNames(t, "the master's entries", master, true, hiddenRaceName, hiddenFeatName, tb.race, tb.feat, tb.trait)
	wantNames(t, "the player's entries", player, false, hiddenWords...)
	wantNames(t, "the player's entries", player, false, tb.race, tb.feat, tb.trait, onlyHiddenName)
	wantNames(t, "the player's entries", player, true, mixedName, lacksTraitName, openFeatName)
}

func TestAPlayersFeatCardsAtCreationNameNoHiddenEntry(t *testing.T) {
	t.Parallel()
	tb := newHiddenNamesTable(t)
	sheet := pensantusSheet()
	sheet.GetFull().Background = &charactersv1.FullSheet_BackgroundKey{BackgroundKey: tb.background}
	preview := func(u *user, kind charactersv1.CharacterKind) *charactersv1.PreviewChoicesResponse {
		res, err := u.api.PreviewChoices(t.Context(), connect.NewRequest(&charactersv1.PreviewChoicesRequest{
			CampaignId: tb.campaign, Kind: kind, Sheet: sheet,
		}))
		if err != nil {
			t.Fatalf("PreviewChoices() error = %v", err)
		}
		return res.Msg
	}
	master := preview(tb.master, charactersv1.CharacterKind_CHARACTER_KIND_ENEMY)
	player := preview(tb.owner, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER)
	// The master reads the feat that names the hidden race and the hidden feat; the player, neither the names nor the feat of the race.
	wantNames(t, "the master's cards", master, true, mixedName, onlyHiddenName, hiddenFeatName, hiddenRaceName)
	wantNames(t, "the player's cards", player, false, hiddenWords...)
	wantNames(t, "the player's cards", player, false, tb.race, tb.feat, onlyHiddenName)
	wantNames(t, "the player's cards", player, true, mixedName, openFeatName, "Pede: ser "+visibleRaceName+" e uma condição que a mesa não mostra.", "Só para "+visibleRaceName+".")
}

// A hidden owner hides its features and options from a player, except where the player's own sheet has the owner
// (a retired class keeps working for who already has it) and the option itself is not retired.
func TestAnOptionOfAHiddenOwnerIsKeptWhereTheSheetHasTheOwner(t *testing.T) {
	t.Parallel()
	srd, err := rules.LoadSRD()
	if err != nil {
		t.Fatal(err)
	}
	const feature = "feature:fighter-fighting-style-defense"
	content, err := srd.With(rules.Overlay{Revision: 1, Off: []string{"class:fighter"}})
	if err != nil {
		t.Fatal(err)
	}
	fighter := levelUpTarget{content: content, build: rules.Build{Classes: []rules.ClassLevel{{Class: "class:fighter", Level: 2}}}}
	wizard := levelUpTarget{content: content, build: rules.Build{Classes: []rules.ClassLevel{{Class: "class:wizard", Level: 2}}}}
	if !content.Hidden(feature) {
		t.Fatalf("Hidden(%s) = false with the fighter off", feature)
	}
	if hiddenOption(fighter, feature) {
		t.Errorf("the fighter's own option %s is hidden from the fighter", feature)
	}
	if !hiddenOption(wizard, feature) {
		t.Errorf("the fighter's option %s is not hidden from a wizard", feature)
	}
	// In the listing, what a sheet's entry owns is read by that player and not by another.
	kept := (&tableListing{content: content, hidden: content.Hidden, mine: map[string]bool{"class:fighter": true}}).hiddenIn()
	other := (&tableListing{content: content, hidden: content.Hidden, mine: map[string]bool{"class:wizard": true}}).hiddenIn()
	if kept(feature) || !other(feature) {
		t.Errorf("hidden for the fighter = %v, for the wizard = %v; want false and true", kept(feature), other(feature))
	}
}

// An option the master switched off is named, in no payload, to a fighter who never picked it; the master reads it (the sheet that picked it is the rules test's control).
func TestASwitchedOffOptionIsNotNamedToAFighterWhoNeverPickedIt(t *testing.T) {
	t.Parallel()
	tb := newLevelUpTable(t, 2700)
	m := tb.master
	setRules(t, m, tb.campaign, func(r *campaignsv1.TableRules) { r.FeatsAllowed = true })
	const name, stem = "Guarda Dupla de Teste", "guarda-dupla-de-teste"
	option := m.addEntry(t, tb.campaign, &rulesv1.TableOption{NamePt: name, Set: "fighting_style"}).GetKey()
	feat := m.addEntry(t, tb.campaign, &rulesv1.TableFeat{NamePt: "Sem guarda", DescPt: []string{"Texto."}, Prerequisite: &rulesv1.FeatPrerequisite{LacksFeatureKey: option}}).GetKey()
	m.setOff(t, tb.campaign, true, option)
	fighter := tb.other.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Toren", levelUpFighterSheet(3, 2700, "subclass:champion"))
	player, err := tb.options(tb.other, fighter)
	if err != nil {
		t.Fatal(err)
	}
	master, err := tb.options(m, fighter)
	if err != nil {
		t.Fatal(err)
	}
	wantNames(t, "the master's options", master, true, name, stem)
	wantNames(t, "the fighter's options", player, false, name, stem)
	if optionOf(player, feat) == nil {
		t.Error("the fighter is not offered the feat that asks to lack the option")
	}
}
