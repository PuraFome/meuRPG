# Performance budgets

The server is small (see [Operations](operations.md#hosting) for its CPU, memory and `GOMEMLIMIT`), so the work that can grow with user input has a budget in code.
This page lists each budget and where it is set, then the commands that measure the expensive paths.
No timings are recorded here: they depend on the machine, so measure on yours with the commands below.
How the deployed numbers add up against the memory limit is in [Operations](operations.md#fog-image-tiles).

## Budgets the code enforces

| Budget | Value | Set in |
| --- | --- | --- |
| Uploaded image file | 10 MiB, before and after re-encoding | `images.MaxBytes`, `backend/internal/maps/images/images.go` |
| Uploaded image size | 8,192 px per side and 40 megapixels | `images.MaxSide`, `images.MaxPixels` |
| Decode memory of an upload | 256 MiB estimated; more is refused as `DIMENSIONS` | `maxDecodeBytes` in `images.go` |
| Decode, resize and crop of a model answer | 178 MiB | `maxFitBytes` in `images.go` |
| Image processing slot | One image decoded at a time on the server | `processing` in `backend/internal/maps` |
| Generated images | One call to the model at a time; an answer of at most 8 MiB | `maxGenerating` in `imagegen.go`; `maxResponseBytes` in `images/gen/gemini.go` |
| Fog tile size | 16 x 16 squares | `tileSquares` in `backend/internal/maps/tiles.go` |
| Working copy of a fogged map image | Long side capped at 2,048 px; two maps at once, least recently used first | `workingMaxSide`, `workingCopies` |
| Finished-tile cache | 32 MiB of PNG bytes, counted in bytes | `tileCacheBytes` |
| Image refused for a tile | Decoding would take more than 192 MiB (answered `503`) | `decodeLimit` |
| One render at a time | A request waits 8 s for its turn, then gets `503` with `Retry-After`; a user's cache misses are also rate limited | `renderWait`, `retryAfterSeconds`, `tileRenderer.gate` |
| Compiled fog scenes | 8 scenes, 24 viewers' views each | `litCacheSize`, `viewsPerScene` in `maps/fog.go` |
| Live table contents | 8 contents and 24 MiB of stored data; the newest always stays | `maxLiveContents`, `maxLiveBytes` in `backend/internal/characters/tablesource.go` |
| Table-content entries | 300 per campaign, 64 KiB each | `rules.MaxOverlayEntries`, `characters.MaxTableEntryBytes` |
| Content catalogs | 8, holding their content weakly | `maxCatalogs` in `backend/internal/characters/contentsource.go` |
| Generated-image requests kept alive | 5 | `maxPendingRequests` in `backend/internal/maps/imagegen.go` |
| One table overlay | 2,000 effects, 500 distinct formulas, 60 traits per race, 20 effects per feature | `MaxOverlayEffects`, `MaxOverlayFormulas`, `MaxTraits`, `MaxFeatureEffects` in `backend/internal/rules/overlay_budget.go` |
| Database pool | 10 connections in production | `defaultMaxConns` in `backend/internal/platform/db/db.go` |
| Dungeon size | 15 to 199 wide, 15 to 399 high | `Options` in `backend/internal/rules/dungeon/options.go` |
| Web initial bundle | 500 kB (warning from 490 kB) | `web/angular.json`; see [CI](ci.md#the-web-bundle-budget) |

Request rate limits and per-account caps are in [Architecture](architecture.md#abuse-limits).
Nothing the tile and scene caches hold is stored on disk or in the database: a restart empties them and they rebuild themselves.

## Time budgets the tests check

Some tests assert a time budget, but only with `MEURPG_MEASURE=1`, without `-race` and on a quiet machine: on a busy machine or in CI a clock test fails for nothing.

- Building a table overlay with `Content.With` at every budget stays under 50 ms (`TestWithAtTheBudgets`).
- The dungeon generator at its largest size and `PromptOf` in the worst case stay inside the budgets stated in `backend/internal/rules/dungeon` (`BenchmarkGenerate`, `BenchmarkWorst`, `BenchmarkPromptOf`).
- Fog vision for the 200 x 400 map has a budget in `backend/internal/rules/vision/bench_test.go`.

## Commands that measure

Run them from `backend/`, without `-race` (the detector multiplies time and memory). Tests that need a database take `MEURPG_TEST_DATABASE_URL`, see [Testing](testing.md#integration-tests).

| What | Command |
| --- | --- |
| The rules engine, one `Derive` (runs on every sheet read) | `go test ./internal/rules -run '^$' -bench Derive -benchmem` |
| Loading the SRD | `go test ./internal/rules -run '^$' -bench LoadSRD -benchmem` |
| A table overlay | `go test ./internal/rules -run '^$' -bench BenchmarkWith -benchmem`; memory: `MEURPG_MEASURE=1 go test ./internal/rules -run TestWithMemory -v`; at the budgets: `MEURPG_MEASURE=1 go test ./internal/rules -run TestWithAtTheBudgets -v` |
| Fog vision (cave, medium and largest maps, dark and lit) | `go test ./internal/rules/vision -run '^$' -bench . -benchmem` |
| A player's filtered map read, without the database | `go test -run '^$' -bench FilteredMap -benchmem ./internal/maps` |
| The whole `GetMap` with fog, with the database | `go test -run TestFogGetMapTiming -v ./internal/maps` |
| Memory of compiled scenes | `go test -run TestSceneMemory -v ./internal/maps` |
| Fog tiles: cold, warm, JPEG | `go test -run '^$' -bench Tile -benchmem ./internal/maps` |
| Tile memory and a request through the whole route | `MEURPG_MEASURE=1 go test -run 'TestTileMemory\|TestTileRequestTiming' -v ./internal/maps` |
| Dungeon generator, from 31 x 31 to 199 x 399, and the constructed worst cases | `go test ./internal/rules/dungeon -run '^$' -bench 'Generate\|Worst\|PromptOf' -benchmem` |
| Drawing a dungeon | `go test -run '^$' -bench Render ./internal/maps/dungeonimg` and `go test -run '^$' -bench DungeonImage -benchmem ./internal/maps` |
| The whole `CreateDungeonMap` call | `go test -run TestDungeonMapCreationTiming -v ./internal/maps` |
| The drawing sent to the image model, biggest map | `MEURPG_MEASURE=1 go test -run TestMeasureTheBiggestMap -v ./internal/maps/refimg ./internal/maps` and `go test -bench BiggestMap -benchmem ./internal/maps/refimg` |
| Image resizing and the worst-case crop | `MEURPG_MEASURE=1 go test -run 'TestShrinkMemory\|TestCropFitMemory' -v ./internal/maps/images` |
| Image generation answers | `MEURPG_MEASURE=1 go test -run TestMeasureMemory -v ./internal/maps/images/gen` |
| Treasure and encounter generators | `go test ./internal/rules -run '^$' -bench 'GenerateTreasure\|GenerateEncounter' -benchmem` |
| Puzzle maths | `go test ./internal/rules/puzzle -run '^$' -bench .` |

### The map editor in the browser

Painting on a big map must not freeze the screen: `PaintedLayers` (`web/src/app/core/maps/paint-layers.ts`) decodes the whole grid once per animation frame (`requestAnimationFrame`), not on every square the pointer crosses, and batches go to the server outside the drawing path (see [Architecture](architecture.md#map-editor)).
To measure it, write a Playwright test that creates the largest map the server accepts (200 x 400 squares), enters "Pintar", picks the wall and the 3 x 3 brush, and records the duration of each `requestAnimationFrame` while `page.mouse` drags across the map; a frame budget is 16.7 ms.
