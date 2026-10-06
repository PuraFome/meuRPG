package puzzle

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestFold(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"Sombra", "sombra"},
		{"  A  SOMBRA!  ", "a sombra"},
		{"Escuridão", "escuridao"},
		{"CORAÇÃO", "coracao"},
		{"pé-de-vento", "pe de vento"},
		{"Ação, às vezes.", "acao as vezes"},
		{"42", "42"},
		{"???", ""},
		{"", ""},
		{"Cañón", "canon"},
	}
	for _, tt := range tests {
		if got := Fold(tt.in); got != tt.want {
			t.Errorf("Fold(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMatches(t *testing.T) {
	t.Parallel()
	answers := []string{"sombra", "a sombra", "Eco"}
	for _, ok := range []string{"sombra", "SOMBRA", "  A Sombra ", "a sombra!", "ECO", "éco"} {
		if !Matches(answers, ok) {
			t.Errorf("Matches(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"escuridão", "sombras", "", "   ", "a  a sombra", "som bra"} {
		if Matches(answers, bad) {
			t.Errorf("Matches(%q) = true, want false", bad)
		}
	}
	if Matches([]string{"?"}, "?") {
		t.Error("an answer with nothing in it matched")
	}
}

func TestValidateAnswers(t *testing.T) {
	t.Parallel()
	many := make([]string, MaxAnswers+1)
	for i := range many {
		many[i] = strings.Repeat("a", i+1)
	}
	tests := []struct {
		name string
		in   []string
		ok   bool
	}{
		{"one", []string{"sombra"}, true},
		{"ten", many[:MaxAnswers], true},
		{"none", nil, false},
		{"eleven", many, false},
		{"empty", []string{"  "}, false},
		{"punctuation only", []string{"?!"}, false},
		{"too long", []string{strings.Repeat("a", MaxAnswerLen+1)}, false},
		{"longest", []string{strings.Repeat("a", MaxAnswerLen)}, true},
		{"twice once folded", []string{"Sombra", "sombra!"}, false},
	}
	for _, tt := range tests {
		err := ValidateAnswers(tt.in)
		if (err == nil) != tt.ok || (err != nil && !errors.Is(err, ErrAnswers)) {
			t.Errorf("%s: ValidateAnswers() error = %v, want ok = %v", tt.name, err, tt.ok)
		}
	}
}

func TestCipherTheArtboardsLetter(t *testing.T) {
	t.Parallel()
	got, err := Encipher("O tesouro está sob o altar", CipherKey{Shift: 3})
	if err != nil {
		t.Fatal(err)
	}
	if want := "R WHVRXUR HVWD VRE R DOWDU"; got != want {
		t.Fatalf("Encipher() = %q, want %q", got, want)
	}
}

func TestCipherRoundTrip(t *testing.T) {
	t.Parallel()
	keys := []CipherKey{{Shift: 1}, {Shift: 3}, {Shift: 13}, {Shift: 25}, {Keyword: "lua"}, {Keyword: "Coruja Negra"}, {Keyword: "zyx"}}
	for _, msg := range []string{"O tesouro está sob o altar", "Sob a LUA, 3 passos à esquerda: ação!", "x"} {
		for _, k := range keys {
			enc, err := Encipher(msg, k)
			if err != nil {
				t.Fatalf("Encipher(%q, %+v) error = %v", msg, k, err)
			}
			if strings.ContainsAny(enc, "abcdefghijklmnopqrstuvwxyz") {
				t.Errorf("Encipher(%q) = %q has a plain letter", msg, enc)
			}
			dec, err := Decipher(enc, k)
			if err != nil {
				t.Fatal(err)
			}
			if Fold(dec) != Fold(msg) {
				t.Errorf("round trip of %q with %+v = %q", msg, k, dec)
			}
			// Everything that is not a letter stays where it was.
			if len([]rune(enc)) != len([]rune(strings.TrimSpace(msg))) {
				t.Errorf("Encipher(%q) changed the length: %q", msg, enc)
			}
		}
	}
}

func TestCipherKeyedAlphabet(t *testing.T) {
	t.Parallel()
	got, err := Encipher("abcde", CipherKey{Keyword: "lua"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "LUABC" {
		t.Errorf("Encipher(abcde, lua) = %q, want LUABC", got)
	}
	// A keyword's letters count once, in order, accents off.
	a, _ := Encipher("abcde", CipherKey{Keyword: "Lúa-lua"})
	if a != got {
		t.Errorf("a keyword with a repeat and an accent gave %q, want %q", a, got)
	}
}

func TestCipherRefusals(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		msg  string
		key  CipherKey
	}{
		{"no key", "oi", CipherKey{}},
		{"shift 0 and keyword", "oi", CipherKey{Shift: 3, Keyword: "lua"}},
		{"shift 26", "oi", CipherKey{Shift: 26}},
		{"negative shift", "oi", CipherKey{Shift: -3}},
		{"short keyword", "oi", CipherKey{Keyword: "lu"}},
		{"keyword of repeats", "oi", CipherKey{Keyword: "lllll"}},
		{"identity keyword", "oi", CipherKey{Keyword: "abc"}},
		{"empty message", " ", CipherKey{Shift: 3}},
		{"no letter", "123 !?", CipherKey{Shift: 3}},
		{"too long", strings.Repeat("a", MaxCipherMessage+1), CipherKey{Shift: 3}},
	}
	for _, tt := range tests {
		if err := ValidateCipher(tt.msg, tt.key); !errors.Is(err, ErrCipher) {
			t.Errorf("%s: ValidateCipher() error = %v, want ErrCipher", tt.name, err)
		}
	}
	if err := ValidateCipher(strings.Repeat("a", MaxCipherMessage), CipherKey{Shift: 3}); err != nil {
		t.Errorf("the longest message: %v", err)
	}
}

func TestValidateSequence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		bells int
		steps []int
		want  error
	}{
		{"ok", 4, []int{0, 2, 1, 3, 3, 0}, nil},
		{"fewest", 3, []int{0, 1, 2}, nil},
		{"most", 8, []int{0, 1, 2, 3, 4, 5, 6, 7, 0, 1, 2, 3}, nil},
		{"two bells", 2, []int{0, 1, 0}, ErrSize},
		{"nine bells", 9, []int{0, 1, 0}, ErrSize},
		{"two steps", 4, []int{0, 1}, ErrSize},
		{"thirteen steps", 4, make([]int, 13), ErrSize},
		{"a bell that is not there", 4, []int{0, 1, 4}, ErrSymbols},
		{"negative", 4, []int{0, -1, 2}, ErrSymbols},
		{"one bell over and over", 4, []int{2, 2, 2}, ErrSequence},
	}
	for _, tt := range tests {
		err := ValidateSequence(tt.bells, tt.steps)
		if (tt.want == nil && err != nil) || (tt.want != nil && !errors.Is(err, tt.want)) {
			t.Errorf("%s: ValidateSequence() error = %v, want %v", tt.name, err, tt.want)
		}
	}
}

func TestSequenceStrike(t *testing.T) {
	t.Parallel()
	steps := []int{2, 0, 1}
	next, wrong, solved, step, err := SequenceStrike(steps, 0, 2)
	if err != nil || next != 1 || wrong || solved || step != 1 {
		t.Fatalf("a right first bell: next=%d wrong=%v solved=%v step=%d err=%v", next, wrong, solved, step, err)
	}
	// A wrong bell, anywhere in the attempt, sends it back to the start.
	next, wrong, solved, step, err = SequenceStrike(steps, 2, 0)
	if err != nil || next != 0 || !wrong || solved || step != 3 {
		t.Fatalf("a wrong third bell: next=%d wrong=%v solved=%v step=%d err=%v", next, wrong, solved, step, err)
	}
	// The last right bell solves it.
	next, wrong, solved, step, err = SequenceStrike(steps, 2, 1)
	if err != nil || next != 3 || wrong || !solved || step != 3 {
		t.Fatalf("the last bell: next=%d wrong=%v solved=%v step=%d err=%v", next, wrong, solved, step, err)
	}
	for _, c := range []struct{ progress, bell int }{{-1, 0}, {3, 0}, {0, -1}} {
		if _, _, _, _, err := SequenceStrike(steps, c.progress, c.bell); !errors.Is(err, ErrMove) {
			t.Errorf("SequenceStrike(progress %d, bell %d) error = %v, want ErrMove", c.progress, c.bell, err)
		}
	}
}

func TestSequencePlayback(t *testing.T) {
	t.Parallel()
	const n = 4
	tests := []struct {
		at      time.Duration
		shown   int
		playing bool
		nextIn  time.Duration
	}{
		{0, 1, true, SequenceStep},
		{SequenceStep - time.Millisecond, 1, true, time.Millisecond},
		{SequenceStep, 2, true, SequenceStep},
		{3 * SequenceStep, 4, true, SequenceStep},
		{4*SequenceStep - time.Millisecond, 4, true, time.Millisecond},
		{4 * SequenceStep, 4, false, 0},
		{time.Hour, 4, false, 0},
		{-time.Second, 1, true, SequenceStep},
	}
	for _, tt := range tests {
		shown, playing, nextIn := SequencePlayback(n, tt.at)
		if shown != tt.shown || playing != tt.playing || nextIn != tt.nextIn {
			t.Errorf("SequencePlayback(%d, %v) = %d, %v, %v; want %d, %v, %v", n, tt.at, shown, playing, nextIn, tt.shown, tt.playing, tt.nextIn)
		}
	}
}

func TestBells(t *testing.T) {
	t.Parallel()
	for n := range 10 {
		got := Bells(n)
		if n < MinBells || n > MaxBells {
			if got != nil {
				t.Errorf("Bells(%d) = %v, want none", n, got)
			}
			continue
		}
		if len(got) != n {
			t.Errorf("Bells(%d) has %d bells", n, len(got))
		}
	}
	seen := map[string]bool{}
	for _, b := range Bells(MaxBells) {
		if b.Key == "" || b.NamePT == "" || seen[b.Key] {
			t.Errorf("bell %+v has no key, no name or repeats", b)
		}
		seen[b.Key] = true
	}
}
