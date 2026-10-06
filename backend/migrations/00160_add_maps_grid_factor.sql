-- +goose Up
-- grid_factor is the calibration of a map's grid (MR-025, RN-25, Etapa 10): how
-- many squares of 1.5 m (5 ft) each square of the drawing is worth. 1 is the
-- default and the behaviour before this column existed (a 1.5 m drawing); 2 is
-- 3 m, 3 is 4.5 m, 4 is 6 m. grid_columns stays what the rules use, the engine's
-- columns: the drawn columns times the factor. So every reader of the grid (the
-- layers, the fog, the combat) is unchanged, and the drawing's own columns are
-- grid_columns / grid_factor. The rows are the drawn rows (the image's
-- proportions) times the factor, so they are never stored either. The range is
-- checked in the next migration (00161), one change each.
ALTER TABLE maps
    ADD COLUMN IF NOT EXISTS grid_factor INT4 NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE maps
    DROP COLUMN IF EXISTS grid_factor;
