package dungeon

import "os"

// measuring says whether the tests hold their time budgets: only when asked
// (MEURPG_MEASURE=1) and without -race. On a busy machine, or in CI, a clock
// check fails for nothing; the benchmarks keep the numbers.
func measuring() bool {
	return os.Getenv("MEURPG_MEASURE") == "1" && !raceEnabled
}
