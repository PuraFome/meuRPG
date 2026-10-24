package rules

import (
	"fmt"
	"slices"
	"strings"
)

// The closed menu of what a table feature may do, as data (MR-025,
// ADR-0018, section 4): the effect types and their fields, and every closed list
// a field takes (targets, modes, senses, recharges...) with Portuguese names, so
// the class editor never hard-codes a list. The validators (checkEffect and
// compileEffect) and this menu read the same lists; the tests prove each value
// the menu offers is accepted and that nothing else is.

// MenuType is one effect type of the menu.
type MenuType struct {
	// Type is the effect's `type`.
	Type string
	// NamePT and HintPT are the picker's label and its one-line explanation, in
	// Portuguese ("Só texto" has no type: a feature with no effects).
	NamePT, HintPT string
	// Fields are the fields the type reads, in the order the editor shows them;
	// every other field of the effect is refused.
	Fields []MenuField
	// FeatOnly says only a feat has this effect: the editor of a feature leaves it
	// out.
	FeatOnly bool
}

// MenuField is a field of an effect type.
type MenuField struct {
	// Name is the proto field of TableEffect.
	Name string
	// Required says the effect is refused without it.
	Required bool
	// Kind is how the editor asks: "formula" (an Int formula), "condition" (a Bool
	// formula), "text", "number", "choice" (one of a closed list, named by List),
	// "choices" (several of it), "tags", "resource_ref" or "yes_no" (a switch).
	Kind string
	// List names the closed list of the menu the value comes from (a MenuList's
	// Name), empty when the value is free.
	List string
	// Min and Max bound a number; both zero when it is unbounded.
	Min, Max int
	// AlwaysShown says the editor shows the field even when it is optional and empty:
	// what the effect does lies in it (a ranged_attack's switches, none required alone).
	AlwaysShown bool
}

// MenuValue is a value of a closed list: the key the effect stores and the name
// in Portuguese. Hint is a short explanation where the name alone does not say.
type MenuValue struct {
	Key, NamePT, HintPT string
}

// MenuList is a closed list a field takes.
type MenuList struct {
	// Name is how fields point at it ("modifier_targets", "senses"...).
	Name   string
	Values []MenuValue
}

// MenuOptionSet is an SRD feature that offers options (a fighting style, an
// eldritch invocation): what a "choice" of kind "feature" may list in `from`.
type MenuOptionSet struct {
	// Key and NamePT name the offering feature; Choose is how many the SRD lets
	// the character pick.
	Key, NamePT string
	Choose      int
	Options     []MenuValue
}

// FormulaHelper is a function a formula may call.
type FormulaHelper struct {
	// Call is how it is written ("mod(\"int\")"), Returns what it gives ("number",
	// "text" or "yes/no") and HintPT says what it is for.
	Call, Returns, HintPT string
}

// EffectMenu is the whole menu.
type EffectMenu struct {
	Types []MenuType
	Lists []MenuList
	// OptionSets are the SRD features whose options a choice effect may offer.
	OptionSets []MenuOptionSet
	// Helpers are the functions a formula ("value", "when", "max", "prepared_max")
	// may use. ClassIndexes are the classes `classLevel(...)` takes: the SRD's and
	// the table's, by their key without "class:".
	Helpers      []FormulaHelper
	ClassIndexes []string
	// The limits the server enforces on features and effects.
	MaxFeaturesPerClass, MaxEffectsPerFeature, MaxTagsPerEffect int
	// ExtraAttackMin and ExtraAttackMax bound an extra_attack's count.
	ExtraAttackMin, ExtraAttackMax int
}

// Names of the closed lists of the menu.
const (
	ListModifierTargets    = "modifier_targets"
	ListModifierModes      = "modifier_modes"
	ListProficiencies      = "proficiency_targets"
	ListProficiencyLevels  = "proficiency_levels"
	ListRollModes          = "roll_modes"
	ListRollTargets        = "roll_targets"
	ListSenses             = "senses"
	ListRecharges          = "recharges"
	ListEconomies          = "economies"
	ListManeuverApplies    = "maneuver_applies"
	ListChoiceKinds        = "choice_kinds"
	ListSkills             = "skills"
	ListLanguages          = "languages"
	ListTools              = "tools"
	ListTagPrefixes        = "tag_prefixes"
	ListAbilities          = "abilities"
	ListIncreaseAmounts    = "ability_increase_amounts"
	ListRangedAttacks      = "ranged_attack_applies"
	ListExtraDamageApplies = "extra_damage_applies"
	ListAmbushApplies      = "ambush_applies"
	ListDamageTypes        = "damage_types"
	// ListResistanceConditions are when a resistance effect holds.
	ListResistanceConditions = "resistance_conditions"
)

// menuTypes is the menu's types, in the picker's order, with the copy in
// Portuguese, in our own words. Only the types of overlayEffectTypes appear.
var menuTypes = []MenuType{
	{Type: "modifier", NamePT: "Modificador", HintPT: "Soma, define ou limita um número da ficha: CA, PV, deslocamento, ataque, dano, uma perícia.", Fields: []MenuField{
		{Name: "target", Required: true, Kind: "choice", List: ListModifierTargets},
		{Name: "mode", Required: true, Kind: "choice", List: ListModifierModes},
		{Name: "value", Required: true, Kind: "formula"},
		{Name: "when", Kind: "condition"},
		{Name: "tags", Kind: "tags", List: ListTagPrefixes},
		{Name: "text_pt", Kind: "text"},
	}},
	{Type: "proficiency", NamePT: "Proficiência", HintPT: "Dá proficiência numa perícia, num teste de resistência, numa ferramenta, arma ou armadura, ou na iniciativa.", Fields: []MenuField{
		{Name: "proficiency", Required: true, Kind: "choice", List: ListProficiencies},
		{Name: "level", Kind: "choice", List: ListProficiencyLevels},
		{Name: "when", Kind: "condition"},
		{Name: "text_pt", Kind: "text"},
	}},
	{Type: "resource", NamePT: "Recurso", HintPT: "Um número de usos que a ficha conta e que volta num descanso.", Fields: []MenuField{
		{Name: "resource", Required: true, Kind: "text"},
		{Name: "max", Required: true, Kind: "formula"},
		{Name: "recharge", Required: true, Kind: "choice", List: ListRecharges},
		{Name: "when", Kind: "condition"},
		{Name: "text_pt", Kind: "text"},
	}},
	{Type: "sense", NamePT: "Sentido", HintPT: "Dá visão no escuro ou outro sentido, com o alcance em pés.", Fields: []MenuField{
		{Name: "sense", Required: true, Kind: "choice", List: ListSenses},
		{Name: "range_ft", Required: true, Kind: "number", Min: 1},
		{Name: "when", Kind: "condition"},
		{Name: "text_pt", Kind: "text"},
	}},
	{Type: "roll_mode", NamePT: "Vantagem ou desvantagem", HintPT: "Vantagem ou desvantagem em certas rolagens; com uma situação (tags), o app só lembra e o mestre decide.", Fields: []MenuField{
		{Name: "roll", Required: true, Kind: "choice", List: ListRollModes},
		{Name: "targets", Required: true, Kind: "choices", List: ListRollTargets},
		{Name: "tags", Kind: "tags", List: ListTagPrefixes},
		{Name: "when", Kind: "condition"},
		{Name: "text_pt", Kind: "text"},
	}},
	{Type: "grant_action", NamePT: "Ação", HintPT: "Uma ação que o personagem passa a ter na vez dele: ação, ação bônus, reação, livre ou movimento. Pode gastar um uso de um recurso que a entrada define (\"Gasta um uso de\").", Fields: []MenuField{
		{Name: "economy", Required: true, Kind: "choice", List: ListEconomies},
		{Name: "resource", Kind: "resource_ref"},
		{Name: "when", Kind: "condition"},
		{Name: "tags", Kind: "tags", List: ListTagPrefixes},
		{Name: "text_pt", Kind: "text"},
	}},
	{Type: "superiority_die", NamePT: "Dado de manobra", HintPT: "Um dado que uma opção (uma manobra) soma a uma rolagem e que gasta um uso de um recurso que a entrada define: ao dano de um ataque com arma que acertou, a um ataque, ao teste de Atletismo de uma agarrada, como redução do dano de um ataque corpo a corpo (reação), ou ao dano com uma provocação: o alvo faz um teste de Sabedoria contra a CD da manobra (8 + proficiência + o modificador da habilidade escolhida) e, se falhar, tem desvantagem nos ataques contra quem não seja o personagem até o fim do próximo turno dele. O texto é o efeito que o mestre lê.", Fields: []MenuField{
		{Name: "applies", Required: true, Kind: "choice", List: ListManeuverApplies},
		{Name: "resource", Required: true, Kind: "resource_ref"},
		{Name: "value", Required: true, Kind: "formula"},
		{Name: "ability", Kind: "choice", List: ListAbilities},
		{Name: "economy", Kind: "choice", List: ListEconomies},
		{Name: "when", Kind: "condition"},
		{Name: "text_pt", Kind: "text"},
	}},
	{Type: "extra_attack", NamePT: "Ataque extra", HintPT: "Quantos ataques a ação Atacar faz; vale o maior que o personagem tiver.", Fields: []MenuField{
		{Name: "count", Required: true, Kind: "number", Min: 2, Max: 4},
		{Name: "when", Kind: "condition"},
		{Name: "text_pt", Kind: "text"},
	}},
	{Type: "extra_damage", NamePT: "Dano extra", HintPT: "Dano somado a um ataque com arma que acertou, no máximo uma vez por turno, quando o alvo é o que o texto pede (qualquer um, ou maior que o personagem). O valor é uma fórmula. Se a entrada define um recurso, cada uso gasta um (por descanso curto ou longo). O jogador escolhe usar na hora de rolar o dano.", Fields: []MenuField{
		{Name: "applies", Required: true, Kind: "choice", List: ListExtraDamageApplies},
		{Name: "value", Required: true, Kind: "formula"},
		{Name: "resource", Kind: "resource_ref"},
		{Name: "when", Kind: "condition"},
		{Name: "text_pt", Kind: "text"},
	}},
	{Type: "ambush", NamePT: "Emboscada", HintPT: "Uma vantagem do personagem contra alvos pegos de surpresa: vantagem nas jogadas de ataque contra uma criatura que ainda não teve o seu turno no combate, ou acerto crítico em uma criatura surpresa (a marca de surpresa é a do mestre, no início do combate).", Fields: []MenuField{
		{Name: "applies", Required: true, Kind: "choice", List: ListAmbushApplies},
		{Name: "when", Kind: "condition"},
		{Name: "text_pt", Kind: "text"},
	}},
	{Type: "ranged_attack", NamePT: "Ataque à distância", HintPT: "Muda os ataques de longe do personagem: os de magias que pedem jogada de ataque ou os com arma à distância (e a arma arremessada de longe). Pode tirar o bônus de meia cobertura e de três quartos do alvo, multiplicar o alcance dessas magias ou tirar a desvantagem de atirar no alcance longo da arma.", Fields: []MenuField{
		{Name: "applies", Required: true, Kind: "choice", List: ListRangedAttacks},
		{Name: "ignore_cover", Kind: "yes_no", AlwaysShown: true},
		{Name: "range_multiplier", Kind: "number", Min: MinSpellRangeMultiplier, Max: MaxSpellRangeMultiplier, AlwaysShown: true},
		{Name: "no_long_range_disadvantage", Kind: "yes_no", AlwaysShown: true},
		{Name: "when", Kind: "condition"},
		{Name: "text_pt", Kind: "text"},
	}},
	{Type: "resistance", NamePT: "Resistência a dano", HintPT: "Resistência a todos os tipos de dano menos um, que a entrada escolhe, enquanto vale a condição (em fúria). A resistência não se acumula: a mesma de outra fonte conta uma vez só.", Fields: []MenuField{
		{Name: "except", Required: true, Kind: "choice", List: ListDamageTypes},
		{Name: "while", Required: true, Kind: "choice", List: ListResistanceConditions},
		{Name: "text_pt", Kind: "text"},
	}},
	{Type: "choice", NamePT: "Escolha", HintPT: "Algo que o jogador escolhe: perícias, especialização, truques, magias, idiomas, ferramentas ou uma opção de uma lista (do SRD, como um estilo de luta, ou a lista de opções da própria característica, como manobras).", Fields: []MenuField{
		{Name: "choice", Required: true, Kind: "choice", List: ListChoiceKinds},
		{Name: "count", Required: true, Kind: "number", Min: 1, Max: MaxChoiceCount},
		{Name: "from", Kind: "choices"},
		{Name: "when", Kind: "condition"},
		{Name: "text_pt", Kind: "text"},
	}},
	{Type: "ability_increase", NamePT: "Aumento de habilidade", FeatOnly: true, HintPT: "O jogador escolhe quantas habilidades de uma lista e soma o mesmo valor a cada uma, sem passar de 20 (como o meio talento: +1 em uma de duas).", Fields: []MenuField{
		{Name: "count", Required: true, Kind: "number", Min: 1, Max: abilityCount},
		{Name: "from", Required: true, Kind: "choices", List: ListAbilities},
		{Name: "value", Required: true, Kind: "choice", List: ListIncreaseAmounts},
		{Name: "text_pt", Kind: "text"},
	}},
	{Type: "note", NamePT: "Nota e magia concedida", HintPT: "Um lembrete na ficha, com um número se quiser, ou magias que a característica concede (um truque, uma magia por dia).", Fields: []MenuField{
		{Name: "text_pt", Kind: "text"},
		{Name: "value", Kind: "formula"},
		{Name: "spells", Kind: "choices"},
		{Name: "tags", Kind: "tags", List: ListTagPrefixes},
		{Name: "when", Kind: "condition"},
	}},
}

// The Portuguese names of the small closed lists, in our own words.
var (
	modifierTargetNames = map[string]string{
		"ac.base": "Classe de Armadura base", "ac": "Classe de Armadura", "hp.max": "Pontos de vida máximos",
		"speed.walk": "Deslocamento", "initiative": "Iniciativa",
		"attack.weapon.melee": "Ataque com arma corpo a corpo", "attack.weapon.ranged": "Ataque com arma à distância",
		"damage.weapon.melee": "Dano de arma corpo a corpo", "damage.weapon.ranged": "Dano de arma à distância",
	}
	modeNames   = map[string][2]string{"add": {"Somar", "Soma o valor ao número."}, "max": {"Pelo menos", "O número não fica abaixo do valor."}, "set": {"Definir", "O número passa a ser o valor."}}
	levelNames  = map[string]string{"half": "Metade", "full": "Completa", "expertise": "Especialização"}
	rollNames   = map[string]string{"advantage": "Vantagem", "disadvantage": "Desvantagem"}
	senseNames  = map[string]string{"darkvision": "Visão no escuro", "blindsight": "Visão às cegas", "tremorsense": "Sentido sísmico", "truesight": "Visão verdadeira"}
	rechargeNms = map[string]string{"short_rest": "Descanso curto", "long_rest": "Descanso longo", "dawn": "Ao amanhecer", "none": "Não volta sozinho"}
	maneuverNms = map[string]string{
		ManeuverDamage: "Dano de um ataque com arma que acertou", ManeuverAttack: "Rolagem de ataque", ManeuverGrapple: "Atletismo de uma agarrada (ação bônus)",
		ManeuverReduceMelee: "Reduz o dano de um ataque corpo a corpo (reação)",
		ManeuverGoad:        "Dano de um ataque com arma que acertou e provocação (teste de Sabedoria)",
	}
	ambushNms = map[string][2]string{
		AmbushUnactedAdvantage:  {"Vantagem contra quem não agiu", "Vantagem nas jogadas de ataque contra uma criatura que ainda não teve um turno no combate."},
		AmbushSurprisedCritical: {"Crítico contra surpreso", "Um acerto contra uma criatura surpresa é um acerto crítico."},
	}
	extraDamageNms = map[string][2]string{
		ExtraDamageAnyTarget:    {"Qualquer alvo", "Vale contra qualquer criatura atingida."},
		ExtraDamageLargerTarget: {"Alvo maior que o personagem", "Só vale se o alvo for de um tamanho maior que o do personagem."},
	}
	rangedAttackNms = map[string][2]string{
		RangedAttackSpell:  {"Magias com jogada de ataque", "Só as magias que pedem jogada de ataque. O alcance multiplicado vale para as que têm alcance em metros; toque e pessoal não mudam."},
		RangedAttackWeapon: {"Ataques com arma à distância", "Armas à distância, e a arma de arremesso quando atirada de longe (a mais de 1,5 m)."},
	}
	economyNms  = map[string]string{"action": "Ação", "bonus_action": "Ação bônus", "reaction": "Reação", "free": "Livre", "movement": "Movimento"}
	choiceNames = map[string]string{
		"skill": "Perícias", "expertise": "Especialização", "cantrip": "Truques", "spell": "Magias",
		"language": "Idiomas", "tool": "Ferramentas", "feature": "Uma opção de uma lista do SRD",
		"feat": "Talentos",
	}
	rollTargetNames = map[string]string{
		"attack": "Ataques", "death_save": "Testes contra a morte", "initiative": "Iniciativa",
		"check.all": "Todos os testes de habilidade", "save.all": "Todos os testes de resistência",
	}
	tagPrefixes = []MenuValue{
		{"against:", "Contra…", "Só vale contra o que a etiqueta diz (against:magic); o app lembra e o mestre decide."},
		{"about:", "Sobre…", "Vale sobre um assunto (about:magic-items); o app lembra e o mestre decide."},
	}
)

// rangedAttackValues is the list of a ranged_attack's applies, with its words.
func rangedAttackValues() []MenuValue {
	out := make([]MenuValue, 0, len(rangedAttackApplies))
	for _, k := range rangedAttackApplies {
		out = append(out, MenuValue{Key: k, NamePT: rangedAttackNms[k][0], HintPT: rangedAttackNms[k][1]})
	}
	return out
}

// EffectMenu gives the closed menu for this content: the table's classes are in
// ClassIndexes, and everything else is the SRD's.
func (c *Content) EffectMenu() EffectMenu {
	x := c.c
	m := EffectMenu{
		Types:               slices.Clone(menuTypes),
		MaxFeaturesPerClass: MaxTableFeatures, MaxEffectsPerFeature: MaxFeatureEffects, MaxTagsPerEffect: maxEffectList,
		ExtraAttackMin: minExtraAttacks, ExtraAttackMax: maxExtraAttacks,
	}
	m.Lists = x.menuLists()
	m.OptionSets = x.menuOptionSets()
	m.Helpers = menuHelpers()
	for k := range x.classes {
		m.ClassIndexes = append(m.ClassIndexes, strings.TrimPrefix(k, "class:"))
	}
	slices.Sort(m.ClassIndexes)
	return m
}

// menuAbilityName is an ability's Portuguese name.
func (c *content) menuAbilityName(a Ability) string { return c.namePT(string(a)) }

// menuSkillRolls are the skills as modifier or roll targets ("skill.stealth").
func (c *content) menuSkillRolls() []MenuValue {
	var out []MenuValue
	for _, k := range c.skillOrder {
		out = append(out, MenuValue{Key: "skill." + strings.TrimPrefix(k, "skill:"), NamePT: "Perícia: " + c.namePT(k)})
	}
	return out
}

// menuSaves are the six saving throws as targets ("save.dex").
func (c *content) menuSaves() []MenuValue {
	var out []MenuValue
	for _, a := range AllAbilities() {
		out = append(out, MenuValue{Key: "save." + string(a), NamePT: "Teste de resistência de " + c.menuAbilityName(a)})
	}
	return out
}

// menuModifierTargets are the fixed targets, then the skills, then the saves.
func (c *content) menuModifierTargets() []MenuValue {
	var targets []MenuValue
	for _, k := range modifierTargets {
		targets = append(targets, MenuValue{Key: k, NamePT: modifierTargetNames[k]})
	}
	targets = append(targets, c.menuSkillRolls()...)
	return append(targets, c.menuSaves()...)
}

// menuRollTargets are the fixed rolls, the ability checks, the skills and the
// saves.
func (c *content) menuRollTargets() []MenuValue {
	var rolls []MenuValue
	for _, k := range []string{"attack", "death_save", "initiative", "check.all", "save.all"} {
		rolls = append(rolls, MenuValue{Key: k, NamePT: rollTargetNames[k]})
	}
	for _, a := range AllAbilities() {
		rolls = append(rolls, MenuValue{Key: "check." + string(a), NamePT: "Testes de " + c.menuAbilityName(a)})
	}
	rolls = append(rolls, c.menuSkillRolls()...)
	return append(rolls, c.menuSaves()...)
}

// menuProficiencies are what a proficiency effect may name, with the skills
// and the tools on their own (in the content's order: the caller sorts them).
func (c *content) menuProficiencies() (profs, skills, tools []MenuValue) {
	profs = append(profs, MenuValue{Key: "skill:*", NamePT: "Todas as perícias"}, MenuValue{Key: "initiative", NamePT: "Iniciativa"})
	for _, k := range c.skillOrder {
		v := MenuValue{Key: k, NamePT: c.namePT(k)}
		skills = append(skills, v)
		profs = append(profs, v)
	}
	profs = append(profs, c.menuSaves()...)
	for _, k := range sortedKeys(c.proficiencies) {
		switch c.proficiencies[k].Kind {
		case "armor", "weapon":
			profs = append(profs, MenuValue{Key: k, NamePT: c.proficiencyNamePT(k)})
		case "tool", "other":
			v := MenuValue{Key: k, NamePT: c.proficiencyNamePT(k)}
			profs = append(profs, v)
			tools = append(tools, v)
		}
	}
	return profs, skills, tools
}

// sortMenuByName sorts values by their Portuguese name, ignoring case.
func sortMenuByName(vs []MenuValue) {
	slices.SortStableFunc(vs, func(a, b MenuValue) int { return strings.Compare(strings.ToLower(a.NamePT), strings.ToLower(b.NamePT)) })
}

// namedMenu gives each key its name.
func namedMenu(keys []string, name func(string) string) []MenuValue {
	out := make([]MenuValue, 0, len(keys))
	for _, k := range keys {
		out = append(out, MenuValue{Key: k, NamePT: name(k)})
	}
	return out
}

// menuLists are the menu's closed lists, in the order the editor shows them.
func (c *content) menuLists() []MenuList {
	profs, skills, tools := c.menuProficiencies()
	var languages []MenuValue
	for _, k := range sortedKeys(c.languages) {
		languages = append(languages, MenuValue{Key: k, NamePT: c.namePT(k)})
	}
	sortMenuByName(skills)
	sortMenuByName(languages)
	sortMenuByName(tools)
	var abilityValues []MenuValue
	for _, a := range AllAbilities() {
		abilityValues = append(abilityValues, MenuValue{Key: string(a), NamePT: c.menuAbilityName(a)})
	}
	var modes []MenuValue
	for _, k := range modifierModes {
		modes = append(modes, MenuValue{Key: k, NamePT: modeNames[k][0], HintPT: modeNames[k][1]})
	}
	return []MenuList{
		{Name: ListModifierTargets, Values: c.menuModifierTargets()},
		{Name: ListModifierModes, Values: modes},
		{Name: ListProficiencies, Values: profs},
		{Name: ListProficiencyLevels, Values: namedMenu(proficiencyLevels, func(k string) string { return levelNames[k] })},
		{Name: ListRollModes, Values: namedMenu(rollModes, func(k string) string { return rollNames[k] })},
		{Name: ListRollTargets, Values: c.menuRollTargets()},
		{Name: ListSenses, Values: namedMenu(senses, func(k string) string { return senseNames[k] })},
		{Name: ListRecharges, Values: namedMenu(recharges, func(k string) string { return rechargeNms[k] })},
		{Name: ListEconomies, Values: namedMenu(economies, func(k string) string { return economyNms[k] })},
		{Name: ListManeuverApplies, Values: namedMenu(maneuverApplies, func(k string) string { return maneuverNms[k] })},
		{Name: ListChoiceKinds, Values: namedMenu(overlayChoiceKinds, func(k string) string { return choiceNames[k] })},
		{Name: ListSkills, Values: skills},
		{Name: ListLanguages, Values: languages},
		{Name: ListTools, Values: tools},
		{Name: ListTagPrefixes, Values: tagPrefixes},
		{Name: ListAbilities, Values: abilityValues},
		{Name: ListIncreaseAmounts, Values: []MenuValue{{Key: "1", NamePT: "+1"}, {Key: "2", NamePT: "+2"}}},
		{Name: ListRangedAttacks, Values: rangedAttackValues()},
		{Name: ListExtraDamageApplies, Values: extraDamageValues()},
		{Name: ListAmbushApplies, Values: ambushValues()},
		{Name: ListDamageTypes, Values: c.menuDamageTypes()},
		{Name: ListResistanceConditions, Values: []MenuValue{{Key: ResistanceWhileRage, NamePT: "Em fúria", HintPT: "Vale enquanto o personagem estiver em fúria (e não usar armadura pesada)."}}},
	}
}

// menuDamageTypes are the damage types, by Portuguese name.
func (c *content) menuDamageTypes() []MenuValue {
	_, _, types := c.namedEntries()
	out := make([]MenuValue, 0, len(types))
	for _, t := range types {
		out = append(out, MenuValue{Key: t.Key, NamePT: t.NamePT})
	}
	return out
}

// menuOptionSets are the SRD features that offer options, sorted by name. A
// set with no option names is left out.
func (c *content) menuOptionSets() []MenuOptionSet {
	var sets []MenuOptionSet
	for _, k := range sortedKeys(c.features) {
		f := c.features[k]
		if len(f.Options) == 0 {
			continue
		}
		set := MenuOptionSet{Key: k, NamePT: c.namePT(k), Choose: f.OptionsChoose}
		for _, o := range f.Options {
			set.Options = append(set.Options, MenuValue{Key: o, NamePT: c.namePT(o)})
		}
		sets = append(sets, set)
	}
	c.labelMenuOptionSets(sets)
	slices.SortStableFunc(sets, func(a, b MenuOptionSet) int { return strings.Compare(a.NamePT, b.NamePT) })
	return sets
}

// labelMenuOptionSets makes two sets with one name (the Fighter's and the
// Paladin's fighting styles) say whose they are, and, when the class repeats
// it too (the Sorcerer's three Metamagic), at which level.
func (c *content) labelMenuOptionSets(sets []MenuOptionSet) {
	names := map[string]int{}
	for _, set := range sets {
		names[set.NamePT]++
	}
	withClass := map[string]int{}
	for _, set := range sets {
		if names[set.NamePT] > 1 {
			withClass[set.NamePT+" ("+c.namePT(c.features[set.Key].Class)+")"]++
		}
	}
	for i, set := range sets {
		if names[set.NamePT] < 2 {
			continue
		}
		f := c.features[set.Key]
		label := set.NamePT + " (" + c.namePT(f.Class)
		if withClass[label+")"] > 1 {
			label += fmt.Sprintf(", nível %d", f.Level)
		}
		sets[i].NamePT = label + ")"
	}
}

// menuHelpers are the functions a formula may call, with their hints.
func menuHelpers() []FormulaHelper {
	return []FormulaHelper{
		{Call: "level()", Returns: "number", HintPT: "O nível total do personagem."},
		{Call: `classLevel("<classe>")`, Returns: "number", HintPT: "O nível numa classe, pelo índice dela (sem o \"class:\"); 0 se o personagem não a tem."},
		{Call: `mod("<habilidade>")`, Returns: "number", HintPT: `O modificador de uma habilidade: "str", "dex", "con", "int", "wis" ou "cha".`},
		{Call: `score("<habilidade>")`, Returns: "number", HintPT: "O valor de uma habilidade, com os bônus."},
		{Call: "prof()", Returns: "number", HintPT: "O bônus de proficiência."},
		{Call: "armor()", Returns: "text", HintPT: `A armadura vestida: "none", "light", "medium" ou "heavy".`},
		{Call: "shield()", Returns: "yes/no", HintPT: "Se o personagem carrega um escudo."},
		{Call: "floor(x)", Returns: "number", HintPT: "Arredonda para baixo."},
		{Call: "ceil(x)", Returns: "number", HintPT: "Arredonda para cima."},
		{Call: "min(a, b)", Returns: "number", HintPT: "O menor dos dois."},
		{Call: "max(a, b)", Returns: "number", HintPT: "O maior dos dois."},
	}
}
