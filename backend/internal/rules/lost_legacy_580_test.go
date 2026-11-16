package rules

import "testing"

// Preflight #580, round 2: a row stored in the old wording ("O personagem não cumpre mais o pré-requisito de <nome>:
// ...", as main stored it) is the same issue whatever the entry is called now, and wherever its pick stands.
func TestARowInTheOldWordingIsTheLostIssueWhateverTheEntryIsCalledNow(t *testing.T) {
	t.Parallel()
	const start = "O personagem não cumpre mais o pré-requisito de "
	feat := Issue{Code: IssueFeatPrerequisite, Field: "full.feat_keys[2]", Lost: &LostPrerequisite{Key: "feat:x@mesa", EntryPT: "Novo nome"}}
	choiceFeat := Issue{Code: IssueFeatPrerequisite, Field: "full.feature_choice_keys[1]", Lost: &LostPrerequisite{Key: "feat:x@mesa", EntryPT: "Novo nome"}}
	option := Issue{Code: IssueOptionPrerequisite, Field: "full.feature_choice_keys[1]", Lost: &LostPrerequisite{Key: "feature:o@mesa", EntryPT: "Novo nome", Option: true}}
	cases := []struct {
		name   string
		is     Issue
		stored string
		want   bool
	}{
		{"a feat, as it was named", feat, "feat_prerequisite|full.feat_keys[2]|" + start + "Novo nome: o talento só vale de novo quando cumprir.", true},
		{"a feat, renamed", feat, "feat_prerequisite|full.feat_keys[2]|" + start + "Nome antigo: o talento só vale de novo quando cumprir.", true},
		{"a feat, its pick has moved", feat, "feat_prerequisite|full.feat_keys[0]|" + start + "Nome antigo: o talento só vale de novo quando cumprir.", true},
		{"a feat taken through a choice, stored without the index", choiceFeat, "feat_prerequisite|full.feature_choice_keys|" + start + "Nome antigo: o talento só vale de novo quando cumprir.", true},
		{"an option, renamed", option, "option_prerequisite|full.feature_choice_keys[1]|" + start + "Nome antigo: precisa de nível 5 de Guerreiro.", true},
		{"an option, its pick has moved", option, "option_prerequisite|full.feature_choice_keys[3]|" + start + "Nome antigo: precisa de nível 5 de Guerreiro.", true},
		{"another code", feat, "option_prerequisite|full.feat_keys[2]|" + start + "Nome antigo: o talento só vale de novo quando cumprir.", false},
		{"another list", feat, "feat_prerequisite|full.feature_choice_keys|" + start + "Nome antigo: o talento só vale de novo quando cumprir.", false},
		{"another sentence", feat, "feat_prerequisite|full.feat_keys[2]|O talento escolhido não existe no conteúdo 1.", false},
		{"the identity of an entry", feat, "feat_prerequisite|full.feat_keys|lost:feat:y@mesa", false},
	}
	for _, tc := range cases {
		if got := KnownIssues([]string{tc.stored}, []Issue{tc.is})[0]; got != tc.want {
			t.Errorf("%s: KnownIssues([%q]) = %v, want %v", tc.name, tc.stored, got, tc.want)
		}
	}
}
