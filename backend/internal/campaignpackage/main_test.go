package campaignpackage_test

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
)

// TestMain drops the test databases this package made.
func TestMain(m *testing.M) { dbtest.Main(m) }
