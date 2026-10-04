package play

import (
	"context"
	"errors"
)

// TreasureFoundIn completes noMaps (play_test.go) as a MapKeeper. The summary's
// "Mais tesouro encontrado" is tested in package maps (TestMR032_SummaryCountsTreasureFoundInTheSession),
// where the master can mark and unmark treasures through the MapService.
func (noMaps) TreasureFoundIn(context.Context, string) (map[string]int32, error) {
	return nil, errors.New("not in this test")
}
