package progression

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
)

// TestMain drops the test databases this package made (dbtest reuses them
// from test to test and drops them once, when the run ends).
func TestMain(m *testing.M) { dbtest.Main(m) }
