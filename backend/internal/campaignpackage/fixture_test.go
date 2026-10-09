package campaignpackage_test

import (
	"fmt"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// fixture is a campaign with one of everything a package carries, and the ids
// the tests refer to.
type fixture struct {
	campaign                string
	village, cave           string // the maps
	villageImage, caveImage string
	portrait                string
	taverna, battle         string // the points
	clue                    string
	npc, creatureNpc        string
	reserved                string
}

// must returns the message of a call's answer, and fails the test (by panic,
// which Go reports with the stack) when the call failed. Calling it as
// must(client.Call(ctx, req)) keeps the fixture readable.
func must[T any](r *connect.Response[T], err error) *T {
	if err != nil {
		panic(fmt.Sprintf("fixture call failed: %v", err))
	}
	return r.Msg
}

func (h *harness) buildFixture(master *user) *fixture {
	t, ctx := h.t, h.t.Context()
	f := &fixture{campaign: h.newCampaign(master, "Mirathel")}
	cid := f.campaign

	// The campaign: how the table plays.
	_ = must(master.campaigns.SetTableRules(ctx, connect.NewRequest(&campaignsv1.SetTableRulesRequest{CampaignId: cid, Rules: &campaignsv1.TableRules{
		DiceMode: campaignsv1.DiceMode_DICE_MODE_APP, CombatStartsWithMap: false, FogOnNewMaps: false,
		HitPoints:      campaignsv1.HitPointsRule_HIT_POINTS_RULE_AVERAGE,
		AbilityMethods: &campaignsv1.AbilityMethods{StandardArray: true, PointBuy: false, Rolled_4D6: true, Typed: false},
		Critical:       campaignsv1.CriticalRule_CRITICAL_RULE_MAX_PLUS_ROLL, DeathSaves: campaignsv1.DeathSaveVisibility_DEATH_SAVE_VISIBILITY_OWNER_AND_MASTER,
		HouseRules:     []string{"Beber uma poção é uma ação bônus.", "Ninguém ressuscita no primeiro dia."},
		HiddenAreaHits: campaignsv1.HiddenAreaHitRule_HIDDEN_AREA_HIT_RULE_ASK,
		EnemyReactions: campaignsv1.EnemyReactionsRule_ENEMY_REACTIONS_RULE_ALWAYS,
	}})))

	// The gallery: two map backgrounds and a portrait.
	villageImg := master.upload(cid, "Vila.png", pngImage(t, 800, 600, 10))
	caveImg := master.upload(cid, "Caverna.png", pngImage(t, 600, 600, 90))
	portrait := master.upload(cid, "Retrato.png", pngImage(t, 200, 200, 200))
	f.villageImage, f.caveImage, f.portrait = villageImg.GetId(), caveImg.GetId(), portrait.GetId()

	// The maps, with a grid, painted layers, fog and doors.
	village := must(master.maps.CreateMap(ctx, connect.NewRequest(&mapsv1.CreateMapRequest{CampaignId: cid, Name: "Vila", ImageId: f.villageImage}))).GetMap()
	cave := must(master.maps.CreateMap(ctx, connect.NewRequest(&mapsv1.CreateMapRequest{CampaignId: cid, Name: "Caverna", ImageId: f.caveImage}))).GetMap()
	f.village, f.cave = village.GetId(), cave.GetId()
	_ = must(master.maps.SetMapGrid(ctx, connect.NewRequest(&mapsv1.SetMapGridRequest{CampaignId: cid, MapId: f.village, Columns: 20})))
	_ = must(master.maps.SetMapGrid(ctx, connect.NewRequest(&mapsv1.SetMapGridRequest{CampaignId: cid, MapId: f.cave, Columns: 10})))
	paint := func(mapID string, layer mapsv1.MapLayer, value int32, squares ...[2]int32) {
		req := &mapsv1.PaintMapCellsRequest{CampaignId: cid, MapId: mapID, Layer: layer, Value: value}
		for _, s := range squares {
			req.Squares = append(req.Squares, &mapsv1.MapSquare{Col: s[0], Row: s[1]})
		}
		_ = must(master.maps.PaintMapCells(ctx, connect.NewRequest(req)))
	}
	paint(f.village, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{3, 3}, [2]int32{4, 3}, [2]int32{5, 3})
	paint(f.village, mapsv1.MapLayer_MAP_LAYER_DIFFICULT_TERRAIN, 1, [2]int32{8, 8}, [2]int32{9, 8})
	paint(f.village, mapsv1.MapLayer_MAP_LAYER_COVER, 1, [2]int32{10, 2})
	paint(f.village, mapsv1.MapLayer_MAP_LAYER_DOORS, int32(mapsv1.DoorState_DOOR_STATE_CLOSED), [2]int32{5, 5})
	_ = must(master.maps.SetMapFog(ctx, connect.NewRequest(&mapsv1.SetMapFogRequest{
		CampaignId: cid, MapId: f.cave, FogEnabled: ptr(true), BaseLight: ptr(mapsv1.LightLevel_LIGHT_LEVEL_DIM), GroupVision: ptr(true),
	})))
	_ = must(master.maps.SetMapRevealed(ctx, connect.NewRequest(&mapsv1.SetMapRevealedRequest{CampaignId: cid, MapId: f.village, Revealed: true})))

	// The points: a scene with actions, clues and hooks; a submap, a battle,
	// a trap, a treasure and a light.
	point := func(mapID string, req *mapsv1.CreateMapPointRequest) string {
		req.CampaignId, req.MapId = cid, mapID
		return must(master.maps.CreateMapPoint(ctx, connect.NewRequest(req))).GetPoint().GetId()
	}
	f.taverna = point(f.village, &mapsv1.CreateMapPointRequest{
		Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "Taverna do Corvo", Description: "Cheira a lenha.", XBp: 1200, YBp: 3400,
		Hooks: "O estalajadeiro sabe do mapa.", ShowDc: true,
	})
	_ = must(master.maps.AddSceneAction(ctx, connect.NewRequest(&mapsv1.AddSceneActionRequest{
		CampaignId: cid, MapId: f.village, PointId: f.taverna, Key: "skill:persuasion", Name: "Convencer o estalajadeiro", Dc: 14, MaxAttempts: ptr(int32(2)),
	})))
	_ = must(master.maps.AddSceneAction(ctx, connect.NewRequest(&mapsv1.AddSceneActionRequest{
		CampaignId: cid, MapId: f.village, PointId: f.taverna, Key: "save:wis", Name: "", Dc: 12,
	})))
	clue := must(master.maps.AddSceneClue(ctx, connect.NewRequest(&mapsv1.AddSceneClueRequest{
		CampaignId: cid, MapId: f.village, PointId: f.taverna, Text: "Há uma marca de corvo na lareira.",
	})))
	f.clue = clue.GetClue().GetId()
	_ = must(master.maps.AddSceneClue(ctx, connect.NewRequest(&mapsv1.AddSceneClueRequest{
		CampaignId: cid, MapId: f.village, PointId: f.taverna, Text: "A chave está sob o balcão.",
	})))
	_ = point(f.village, &mapsv1.CreateMapPointRequest{Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP, Name: "Entrada da caverna", XBp: 8000, YBp: 7000, TargetMapId: f.cave})
	f.battle = point(f.village, &mapsv1.CreateMapPointRequest{Kind: mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE, Name: "Emboscada na ponte", XBp: 500, YBp: 500, TargetMapId: f.cave})
	_ = point(f.village, &mapsv1.CreateMapPointRequest{
		Kind: mapsv1.MapPointKind_MAP_POINT_KIND_TRAP, Name: "Poço escondido", XBp: 2500, YBp: 2500,
		Trap: &mapsv1.TrapSpec{
			NoticeDc: 12, FindDc: 15, AreaSize: 2, Trigger: rulesv1.TrapTrigger_TRAP_TRIGGER_ENTER,
			Effect: &rulesv1.TrapEffect{Damage: []*rulesv1.TrapDamage{{Dice: "2d6", DamageTypeKey: "damage-type:piercing"}}},
		},
	})
	_ = point(f.village, &mapsv1.CreateMapPointRequest{Kind: mapsv1.MapPointKind_MAP_POINT_KIND_TREASURE, Name: "Baú enterrado", XBp: 9000, YBp: 9000, TreasureValuePo: ptr(int32(250))})
	_ = point(f.cave, &mapsv1.CreateMapPointRequest{Kind: mapsv1.MapPointKind_MAP_POINT_KIND_LIGHT, Name: "Tocha do corredor", XBp: 3000, YBp: 3000, Light: &mapsv1.LightSpec{BrightFt: 20, DimFt: 20}})
	revealed := must(master.maps.SetMapPointRevealed(ctx, connect.NewRequest(&mapsv1.SetMapPointRevealedRequest{CampaignId: cid, MapId: f.village, PointId: f.taverna, Revealed: true})))
	_ = revealed

	// The encounter on the battle point.
	hidden := false
	_ = must(master.encounters.SaveBattleEncounter(ctx, connect.NewRequest(&playv1.SaveBattleEncounterRequest{
		CampaignId: cid, MapPointId: f.battle,
		Encounter: &playv1.BattleEncounter{Monsters: []*playv1.MonsterGroup{{CreatureKey: "monster:goblin", Count: 3, Name: "Bandoleiro"}, {CreatureKey: "monster:wolf", Count: 1}}, Hidden: &hidden},
	})))

	// The NPCs: a story NPC with a portrait and notes, and one from a creature.
	story := &charactersv1.BasicSheet{HitPointsMax: 9, ArmorClass: 11, SpeedFt: 30, Description: "A estalajadeira.", PortraitImageId: f.portrait}
	npc := must(master.characters.CreateCharacter(ctx, connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: cid, Kind: charactersv1.CharacterKind_CHARACTER_KIND_STORY, Name: "Dona Lúcia",
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Basic{Basic: story}},
		Story: &charactersv1.CharacterStory{Backstory: "Viu o corvo chegar."},
	})))
	f.npc = npc.GetCharacter().GetId()
	_ = must(master.characters.UpdateMasterNotes(ctx, connect.NewRequest(&charactersv1.UpdateMasterNotesRequest{CampaignId: cid, CharacterId: f.npc, Notes: "Trabalha para o conde."})))
	cr := must(master.characters.CreateNpcFromCreature(ctx, connect.NewRequest(&charactersv1.CreateNpcFromCreatureRequest{
		CampaignId: cid, CreatureKey: "monster:goblin", Name: "Gruk", Kind: charactersv1.CharacterKind_CHARACTER_KIND_MINION, IdempotencyKey: "0d6f0b3e-0f3f-4b53-9d2a-2b0f6c0a1111",
	})))
	f.creatureNpc = cr.GetCharacter().GetId()

	// A player's character, made reserved for a player to claim (MR-049), with the master's notes.
	reserved := must(master.characters.CreateCharacter(ctx, connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: cid, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Pensantus", ForPlayer: true,
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 12, Dexterity: 16, Constitution: 15, Intelligence: 16, Wisdom: 13, Charisma: 12},
			RaceKey:    "race:human",
			Classes:    []*charactersv1.ClassLevel{{ClassKey: "class:fighter", Level: 2}},
			ArmorKey:   "equipment:chain-mail", WeaponKeys: []string{"equipment:longsword"},
		}}},
		Story: &charactersv1.CharacterStory{Backstory: "Veio do mar."},
	})))
	f.reserved = reserved.GetCharacter().GetId()
	_ = must(master.characters.UpdateMasterNotes(ctx, connect.NewRequest(&charactersv1.UpdateMasterNotesRequest{CampaignId: cid, CharacterId: f.reserved, Notes: "Esconde um segredo."})))

	// The document links a map, a character and an image.
	_ = must(master.document.UpdateCampaignDocument(ctx, connect.NewRequest(&campaignsv1.UpdateCampaignDocumentRequest{
		CampaignId: cid, Body: fmt.Sprintf("# Mirathel\n\nComece na [Vila](map:%s). Fale com [Dona Lúcia](character:%s).\n\n![A vila](image:%s)\n\nUm link morto: [fantasma](map:%s)\n",
			f.village, f.npc, f.villageImage, "0b6f4a52-3b5e-4c55-9d0b-2a51f0c1e001"),
	})))

	// The table content: a background, a race, a spell, a class; one archived;
	// one option switched off.
	entry := func(req *rulesv1.CreateTableEntryRequest) string {
		req.CampaignId = cid
		return must(master.table.CreateTableEntry(ctx, connect.NewRequest(req))).GetEntry().GetKey()
	}
	background := entry(&rulesv1.CreateTableEntryRequest{Body: &rulesv1.CreateTableEntryRequest_TableBackground{TableBackground: tableBackground("Guarda de farol")}})
	_ = entry(&rulesv1.CreateTableEntryRequest{Body: &rulesv1.CreateTableEntryRequest_TableRace{TableRace: tableRace("Anão do mar")}})
	_ = entry(&rulesv1.CreateTableEntryRequest{Body: &rulesv1.CreateTableEntryRequest_TableClass{TableClass: tableClass("Guardião das marés")}})
	spell := entry(&rulesv1.CreateTableEntryRequest{Body: &rulesv1.CreateTableEntryRequest_TableSpell{TableSpell: tableSpell("Raio de sal", "class:wizard")}})
	_ = must(master.table.ArchiveTableEntry(ctx, connect.NewRequest(&rulesv1.ArchiveTableEntryRequest{CampaignId: cid, Key: spell})))
	_ = background
	_ = must(master.table.SetOptionSwitches(ctx, connect.NewRequest(&rulesv1.SetOptionSwitchesRequest{
		CampaignId: cid, Switches: []*rulesv1.OptionSwitch{{Key: "class:warlock", Off: true}},
	})))

	// Puzzles: lights that open the village door, a riddle that reveals the clue, and one archived.
	lights := must(master.puzzles.CreatePuzzle(ctx, connect.NewRequest(&playv1.CreatePuzzleRequest{
		CampaignId: cid, Name: "Luzes do selo", Config: &playv1.PuzzleConfig{Kind: &playv1.PuzzleConfig_Lights{Lights: &playv1.LightsConfig{Size: 3}}},
		Clue: "Só o selo apagado abre o caminho.", Hints: []string{"Comece pelo centro."},
		OnSolve: &playv1.PuzzleOnSolve{
			Action: playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_OPEN_DOOR, Message: "A porta da vila se abriu.",
			Target: &playv1.PuzzleOnSolve_Door{Door: &playv1.PuzzleDoorTarget{MapId: f.village, Col: 5, Row: 5}},
		},
		OnWrong: &playv1.PuzzleOnWrong{MaxMoves: 30},
	})))
	_ = lights
	_ = must(master.puzzles.CreatePuzzle(ctx, connect.NewRequest(&playv1.CreatePuzzleRequest{
		CampaignId: cid, Name: "Charada da lareira",
		Config:   &playv1.PuzzleConfig{Kind: &playv1.PuzzleConfig_Riddle{Riddle: &playv1.RiddleConfig{Text: "Moro embaixo de cada passo seu, mas nunca peso nada. O que sou?"}}},
		Solution: &playv1.PuzzleSolution{Kind: &playv1.PuzzleSolution_Riddle{Riddle: &playv1.RiddleSolution{Answers: []string{"sombra"}}}},
		OnSolve:  &playv1.PuzzleOnSolve{Action: playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_REVEAL_CLUE, Target: &playv1.PuzzleOnSolve_Clue{Clue: &playv1.PuzzleClueTarget{ClueId: f.clue}}},
	})))
	old := must(master.puzzles.CreatePuzzle(ctx, connect.NewRequest(&playv1.CreatePuzzleRequest{
		CampaignId: cid, Name: "Cofre antigo", Config: &playv1.PuzzleConfig{Kind: &playv1.PuzzleConfig_Lock{Lock: &playv1.LockConfig{Wheels: 4, Alphabet: playv1.PuzzleAlphabet_PUZZLE_ALPHABET_DIGITS}}},
		Solution: &playv1.PuzzleSolution{Kind: &playv1.PuzzleSolution_Lock{Lock: &playv1.LockSolution{Wheels: []int32{7, 3, 5, 1}}}},
		Start:    &playv1.PuzzleState{Kind: &playv1.PuzzleState_Lock{Lock: &playv1.LockState{Wheels: []int32{0, 0, 0, 0}}}},
	})))
	_ = must(master.puzzles.ArchivePuzzle(ctx, connect.NewRequest(&playv1.ArchivePuzzleRequest{CampaignId: cid, PuzzleId: old.GetPuzzle().GetId()})))
	return f
}

func ptr[T any](v T) *T { return &v }

func tableBackground(name string) *rulesv1.TableBackground {
	return &rulesv1.TableBackground{
		NamePt: name, Skills: []string{"skill:insight", "skill:religion"}, Tools: []string{"proficiency:thieves-tools"},
		LanguageChoices: 1, EquipmentPt: "Uma lanterna e um apito.",
		Feature: &rulesv1.TableFeature{NamePt: "Luz-guia", DescPt: []string{"Conhece a rota dos navios."}, Effects: []*rulesv1.TableEffect{{Type: "note"}}},
	}
}

func tableRace(name string) *rulesv1.TableRace {
	return &rulesv1.TableRace{
		NamePt: name, Size: "Medium", SpeedFt: 25, AbilityBonuses: &rulesv1.AbilityScores{Constitution: 1}, ChoiceBonuses: []int32{2, 1},
		DarkvisionFt: 60, Languages: []string{"language:common"}, LanguageChoices: 1,
		Traits: []*rulesv1.TableFeature{{NamePt: "Resistente", Effects: []*rulesv1.TableEffect{{
			Type: "roll_mode", Roll: "advantage", Targets: []string{"save.con"}, Tags: []string{"against:poison"},
		}}}},
	}
}

func tableSpell(name string, classes ...string) *rulesv1.TableSpell {
	return &rulesv1.TableSpell{
		NamePt: name, Level: 1, SchoolKey: "school:evocation",
		CastingTime: &rulesv1.TableSpellCastingTime{Unit: rulesv1.CastingTimeUnit_CASTING_TIME_UNIT_ACTION, Amount: 1},
		Range:       &rulesv1.TableSpellRange{Kind: rulesv1.SpellRangeKind_SPELL_RANGE_KIND_RANGED, DistanceFt: 60},
		Duration:    &rulesv1.TableSpellDuration{Kind: rulesv1.SpellDurationKind_SPELL_DURATION_KIND_INSTANTANEOUS},
		Components:  &rulesv1.TableSpellComponents{Verbal: true, Somatic: true},
		ClassKeys:   classes, DescPt: []string{"Um raio de teste."},
		Target: &rulesv1.TableSpellTarget{Kind: rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_CREATURE},
		Attack: "ranged",
		Damage: []*rulesv1.TableSpellDamage{{DamageTypeKey: "damage-type:force", Dice: "3d6", PerSlotLevel: "1d6"}},
	}
}

func tableClass(name string) *rulesv1.TableClass {
	c := &rulesv1.TableClass{
		NamePt: name, HitDie: 8,
		SavingThrows: []rulesv1.Ability{rulesv1.Ability_ABILITY_CONSTITUTION, rulesv1.Ability_ABILITY_WISDOM},
		SkillChoose:  2, SkillFrom: []string{"skill:arcana", "skill:history", "skill:medicine", "skill:nature", "skill:survival"},
		Proficiencies: []string{"proficiency:light-armor", "proficiency:simple-weapons"},
		Minimums:      &rulesv1.AbilityScores{Constitution: 11},
		SubclassLevel: 3,
	}
	for lvl := 1; lvl <= 20; lvl++ {
		row := &rulesv1.TableClassLevel{}
		switch lvl {
		case 1:
			row.Features = []*rulesv1.TableFeature{
				{NamePt: "Vigor", DescPt: []string{"Mais iniciativa."}, Effects: []*rulesv1.TableEffect{{Type: "modifier", Target: "initiative", Mode: "add", Value: "1"}}},
				{NamePt: "Treino", Effects: []*rulesv1.TableEffect{{Type: "proficiency", Proficiency: "skill:survival"}}},
			}
		case 2:
			row.Features = []*rulesv1.TableFeature{{NamePt: "Surto", Effects: []*rulesv1.TableEffect{
				{Type: "resource", Resource: "surto_teste", Max: "prof()", Recharge: "short_rest"},
			}}}
		}
		c.Levels = append(c.Levels, row)
	}
	return c
}
