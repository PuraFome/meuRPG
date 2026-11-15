package characters

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
)

func TestSteedDismissalReasons(t *testing.T) {
	t.Parallel()
	steed := creatureView{CharacterCreature: charactersdb.CharacterCreature{Bonded: true, Source: creatureSourceSteed}}
	other := creatureView{CharacterCreature: charactersdb.CharacterCreature{Source: "familiar"}}
	cases := []struct {
		name string
		v    creatureView
		how  charactersv1.CreatureDismissal
		want string
		code connect.Code
	}{
		{"a steed dismissed by default keeps its bond", steed, charactersv1.CreatureDismissal_CREATURE_DISMISSAL_UNSPECIFIED, "dismissed", 0},
		{"a steed dismissed keeps its bond", steed, charactersv1.CreatureDismissal_CREATURE_DISMISSAL_DISMISS, "dismissed", 0},
		{"a steed released goes with the owner's reason", steed, charactersv1.CreatureDismissal_CREATURE_DISMISSAL_RELEASE, "owner", 0},
		{"another creature is dismissed as before", other, charactersv1.CreatureDismissal_CREATURE_DISMISSAL_DISMISS, "owner", 0},
		{"another creature cannot be released", other, charactersv1.CreatureDismissal_CREATURE_DISMISSAL_RELEASE, "", connect.CodeInvalidArgument},
	}
	for _, c := range cases {
		got, err := steedDismissal(c.v, "owner", c.how)
		if (err == nil) != (c.code == 0) || (err != nil && connect.CodeOf(err) != c.code) || got != c.want {
			t.Errorf("%s: steedDismissal() = %q, %v; want %q, code %v", c.name, got, err, c.want, c.code)
		}
	}
}
