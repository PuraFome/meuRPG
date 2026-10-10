package rules

import (
	"regexp"
	"strconv"
	"strings"
)

// The SRD writes a creature's languages in English ("Common, Goblin", "telepathy
// 60 ft.", "understands Common but can't speak"). languagesPT says the same line
// in Portuguese, with the official names of the Brazilian editions for the
// languages the characters have (names_pt.json, "language:<key>") and the same
// style for the rest. Words it does not know stay as the SRD has them.

// languageNamesPT maps the lower-case SRD language word to its Portuguese name.
var languageNamesPT = map[string]string{
	"abyssal": "Abissal", "celestial": "Celestial", "common": "Comum", "deep speech": "Dialeto Subterrâneo",
	"draconic": "Dracônico", "dwarvish": "Anão", "elvish": "Élfico", "giant": "Gigante", "gnomish": "Gnômico",
	"goblin": "Goblin", "halfling": "Halfling", "infernal": "Infernal", "orc": "Orc", "primordial": "Primordial",
	"sylvan": "Silvestre", "undercommon": "Subcomum",
	"druidic": "Dialeto Druídico", "thieves' cant": "Gírias de Ladrão",
	"aquan": "Aquan", "auran": "Auran", "ignan": "Ignan", "terran": "Terran",
	"gnoll": "Gnoll", "otyugh": "Otyugh", "sahuagin": "Sahuagin", "sphinx": "Esfinge", "worg": "Worg",
	"blink dog": "Cão Teleportador", "winter wolf": "Lobo do Inverno",
	"giant eagle": "Águia Gigante", "giant elk": "Alce Gigante", "giant owl": "Coruja Gigante",
}

var (
	languageWordRE = func() *regexp.Regexp {
		// Longest first, so "Giant Eagle" wins over "Giant".
		var keys []string
		for k := range languageNamesPT {
			keys = append(keys, regexp.QuoteMeta(k))
		}
		for i := 1; i < len(keys); i++ {
			for j := i; j > 0 && len(keys[j]) > len(keys[j-1]); j-- {
				keys[j], keys[j-1] = keys[j-1], keys[j]
			}
		}
		return regexp.MustCompile(`(?i)\b(` + strings.Join(keys, "|") + `)\b`)
	}()
	langTelepathyRE  = regexp.MustCompile(`(?i)telepathy (\d+) ft\.`)
	langUnderstandRE = regexp.MustCompile(`(?i)understands (.+?) but (?:can't|doesn't) speak(?: it)?`)
	langAnyRE        = regexp.MustCompile(`(?i)any (one|two|four|six) languages?`)
	langAllRE        = regexp.MustCompile(`(?i)^all\b`)
	langAndRE        = regexp.MustCompile(`,? and `)
)

// languageFixedPT are the whole phrases of the SRD's languages line.
var languageFixedPT = []struct{ from, to string }{
	{"works only with creatures that understand", "só funciona com criaturas que entendem"},
	{"plus up to five other languages", "mais até cinco outros idiomas"},
	{"plus any two languages", "mais dois idiomas quaisquer"},
	{"any languages it knew in life", "quaisquer idiomas que conhecia em vida"},
	{"the languages it knew in life", "os idiomas que conhecia em vida"},
	{"one language known by its creator", "um idioma conhecido por seu criador"},
	{"can't speak in boar form", "não fala na forma de javali"},
	{"usually", "geralmente"},
}

var languageAnyPT = map[string]string{
	"one": "um idioma qualquer", "two": "dois idiomas quaisquer", "four": "quatro idiomas quaisquer", "six": "seis idiomas quaisquer",
}

// languageUnderstoodPT are what a creature may understand without speaking,
// other than a list of languages.
var languageUnderstoodPT = map[string]string{
	"the languages of its creator":      "os idiomas de seu criador",
	"all languages it knew in life":     "todos os idiomas que conhecia em vida",
	"all languages it spoke in life":    "todos os idiomas que falava em vida",
	"commands given in any language":    "comandos dados em qualquer idioma",
	"the languages it knew in life":     "os idiomas que conhecia em vida",
	"any languages it knew in life":     "quaisquer idiomas que conhecia em vida",
	"one language known by its creator": "um idioma conhecido por seu criador",
}

// LanguagesPT writes a creature's SRD languages line in Portuguese.
func LanguagesPT(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = langUnderstandRE.ReplaceAllStringFunc(s, func(m string) string {
		x := langUnderstandRE.FindStringSubmatch(m)[1]
		if pt, ok := languageUnderstoodPT[strings.ToLower(x)]; ok {
			return "entende " + pt + ", mas não fala"
		}
		return "entende " + langAndRE.ReplaceAllString(x, " e ") + ", mas não fala"
	})
	for _, f := range languageFixedPT {
		s = replaceFold(s, f.from, f.to)
	}
	s = langAnyRE.ReplaceAllStringFunc(s, func(m string) string {
		return languageAnyPT[strings.ToLower(langAnyRE.FindStringSubmatch(m)[1])]
	})
	s = langAllRE.ReplaceAllString(s, "todos")
	s = langTelepathyRE.ReplaceAllStringFunc(s, func(m string) string {
		ft, _ := strconv.Atoi(langTelepathyRE.FindStringSubmatch(m)[1])
		return "telepatia " + MetersPT(ft)
	})
	return languageWordRE.ReplaceAllStringFunc(s, func(w string) string {
		return languageNamesPT[strings.ToLower(w)]
	})
}

// replaceFold replaces every case-insensitive occurrence of from in s.
func replaceFold(s, from, to string) string {
	return regexp.MustCompile(`(?i)`+regexp.QuoteMeta(from)).ReplaceAllString(s, to)
}
