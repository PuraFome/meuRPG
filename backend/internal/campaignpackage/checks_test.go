package campaignpackage_test

import (
	"testing"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Each kind goes through the checks of the call that makes the same thing by
// hand, and a refusal says which thing and why, in the game's words.
func TestEachKindIsCheckedLikeWhatTheMasterTypes(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	good := master.export(h.smallCampaign(master))
	const (
		pkgProblem = pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE
		invalid    = pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID
		unknown    = pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNKNOWN_CONTENT
		missing    = pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_IN_PACKAGE
	)
	editMap := func(c *crafted, f func(m *pkgv1.PackageMap)) {
		name := c.entryOf(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_MAP, 0)
		var m pkgv1.PackageMap
		if _, ok := mustRead(c, name, &m); !ok {
			t.Fatal("cannot read the map")
		}
		f(&m)
		c.set(name, mustMarshal(t, &m))
	}
	pointID := func(c *crafted) string {
		var m pkgv1.PackageMap
		mustRead(c, c.entryOf(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_MAP, 0), &m)
		return m.GetPoints()[0].GetId()
	}
	mapID := func(c *crafted) string {
		var m pkgv1.PackageMap
		mustRead(c, c.entryOf(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_MAP, 0), &m)
		return m.GetId()
	}
	tests := []struct {
		name   string
		change func(c *crafted)
		kind   pkgv1.PackageProblemKind
		label  string
		reason pkgv1.PackageProblemReason
	}{
		{
			"a point off the map", func(c *crafted) { editMap(c, func(m *pkgv1.PackageMap) { m.Points[0].XBp = 20000 }) },
			pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_SCENE, "A ponte quebrada", invalid,
		},
		{"a trap of a damage type the rules lack", func(c *crafted) {
			editMap(c, func(m *pkgv1.PackageMap) {
				p := m.Points[0]
				p.Kind, p.Hooks, p.ShowDc = mapsv1.MapPointKind_MAP_POINT_KIND_TRAP, "", false
				p.Trap = &mapsv1.TrapSpec{
					NoticeDc: 10, FindDc: 10, AreaSize: 1, Trigger: rulesv1.TrapTrigger_TRAP_TRIGGER_ENTER,
					Effect: &rulesv1.TrapEffect{Damage: []*rulesv1.TrapDamage{{Dice: "1d6", DamageTypeKey: "damage-type:nonsense"}}},
				}
			})
		}, pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_POINT, "A ponte quebrada", unknown},
		{
			"a grid with too few columns", func(c *crafted) { editMap(c, func(m *pkgv1.PackageMap) { m.GridColumns, m.GridFactor = 3, 1 }) },
			pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_MAP, "Vila", invalid,
		},
		{"layers of another grid", func(c *crafted) {
			editMap(c, func(m *pkgv1.PackageMap) {
				m.GridColumns, m.GridFactor, m.Layers = 10, 1, &pkgv1.PackageLayers{Walls: []byte{1, 2, 3}}
			})
		}, pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_MAP, "Vila", invalid},
		{
			"fog on a map without a grid", func(c *crafted) { editMap(c, func(m *pkgv1.PackageMap) { m.FogEnabled = true }) },
			pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_MAP, "Vila", invalid,
		},
		{"a point that leads to a map the package lacks", func(c *crafted) {
			editMap(c, func(m *pkgv1.PackageMap) {
				m.Points[0].Kind, m.Points[0].Hooks, m.Points[0].ShowDc = mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP, "", false
				m.Points[0].TargetMapId = "0b6f4a52-3b5e-4c55-9d0b-2a51f0c1e001"
			})
		}, pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_POINT, "A ponte quebrada", missing},
		{"an encounter of a creature the SRD lacks", func(c *crafted) {
			editMap(c, func(m *pkgv1.PackageMap) {
				m.Points[0].Kind, m.Points[0].Hooks, m.Points[0].ShowDc = mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE, "", false
			})
			c.add(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_ENCOUNTERS, "encounters.json", mustMarshal(t, &pkgv1.PackageEncounters{Encounters: []*pkgv1.PackageEncounter{{
				PointId: pointID(c), Encounter: &playv1.BattleEncounter{Monsters: []*playv1.MonsterGroup{{CreatureKey: "monster:nonexistent", Count: 1}}},
			}}}))
		}, pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_ENCOUNTER, "A ponte quebrada", unknown},
		{"a puzzle that opens a door the map lacks", func(c *crafted) {
			c.add(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_PUZZLE, "puzzles/1.json", mustMarshal(t, &pkgv1.PackagePuzzle{
				Id: "0b6f4a52-3b5e-4c55-9d0b-2a51f0c1e101", Name: "Luzes",
				Config: &playv1.PuzzleConfig{Kind: &playv1.PuzzleConfig_Lights{Lights: &playv1.LightsConfig{Size: 3}}},
				OnSolve: &playv1.PuzzleOnSolve{
					Action: playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_OPEN_DOOR,
					Target: &playv1.PuzzleOnSolve_Door{Door: &playv1.PuzzleDoorTarget{MapId: mapID(c), Col: 1, Row: 1}},
				},
			}))
		}, pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PUZZLE, "Luzes", missing},
		{"an NPC of a class the rules lack", func(c *crafted) {
			c.add(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_NPC, "npcs/1.json", mustMarshal(t, &pkgv1.PackageNpc{
				Id: "0b6f4a52-3b5e-4c55-9d0b-2a51f0c1e201", Name: "Orla", Kind: charactersv1.CharacterKind_CHARACTER_KIND_ENEMY,
				Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
					BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 10, Constitution: 10, Intelligence: 10, Wisdom: 10, Charisma: 10},
					RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:mesa-que-nao-existe@mesa", Level: 1}},
				}}},
			}))
		}, pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_NPC, "Orla", missing},
		{"an NPC whose portrait is not in the package", func(c *crafted) {
			c.add(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_NPC, "npcs/1.json", mustMarshal(t, &pkgv1.PackageNpc{
				Id: "0b6f4a52-3b5e-4c55-9d0b-2a51f0c1e201", Name: "Dona Lúcia", Kind: charactersv1.CharacterKind_CHARACTER_KIND_STORY,
				Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Basic{Basic: &charactersv1.BasicSheet{
					HitPointsMax: 5, ArmorClass: 10, SpeedFt: 30, PortraitImageId: "0b6f4a52-3b5e-4c55-9d0b-2a51f0c1e777",
				}}},
			}))
		}, pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_NPC, "Dona Lúcia", missing},
		{"a table entry whose key does not fit its kind", func(c *crafted) {
			c.add(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_CONTENT, "content/1.json", mustMarshal(t, &pkgv1.PackageContent{Entry: &rulesv1.TableEntry{
				Key: "spell:guarda@mesa", Kind: rulesv1.TableContentKind_TABLE_CONTENT_KIND_BACKGROUND, NamePt: "Guarda",
				Body: &rulesv1.TableEntry_TableBackground{TableBackground: tableBackground("Guarda")},
			}}))
		}, pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_CONTENT, "Guarda", invalid},
		{"an option switched off that nothing offers", func(c *crafted) {
			c.add(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_CONTENT_OPTIONS, "content/options.json", mustMarshal(t, &pkgv1.PackageContentOptions{Disabled: []string{"class:nao-existe"}}))
		}, pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_CONTENT, "class:nao-existe", unknown},
	}
	_ = pkgProblem
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := unpack(t, good)
			tc.change(c)
			p := only(t, master.preview("x.meurpg.zip", c.bytes()))
			if p.GetKind() != tc.kind || p.GetName() != tc.label || p.GetReason() != tc.reason {
				t.Fatalf("problem = %v, want kind %v name %q reason %v", p, tc.kind, tc.label, tc.reason)
			}
		})
	}
}
