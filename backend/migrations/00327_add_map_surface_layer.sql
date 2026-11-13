-- +goose Up
-- The "surface" layer of a map (#589): two bits a square, in the byte layout of package
-- rules/grid (layer.go, like the cover): 0 floor, 1 water, 2 climbable. A creature that crosses
-- water pays 1 extra foot per foot unless it has a swimming speed, and one that crosses a climbable
-- surface unless it has a climbing speed (SRD 5.1). NULL when nothing is painted; a new grid or a
-- new image deletes it with the rest of the row.
ALTER TABLE map_layers
    ADD COLUMN IF NOT EXISTS surface BYTEA NULL;

-- +goose Down
ALTER TABLE map_layers
    DROP COLUMN IF EXISTS surface;
