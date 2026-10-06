-- +goose Up
-- Images made before the column existed: the textured maps by their request, and every edit below them, however deep the
-- chain goes (one recursive pass from the textured pictures down through parent_image_id). Safe to run again: it only
-- fills what is still empty.
UPDATE gallery_images SET generated_kind = 'textured_map'
WHERE generated_kind = '' AND id IN (
    WITH RECURSIVE chain AS (
        SELECT g.id FROM gallery_images AS g
        JOIN image_requests AS r ON r.image_id = g.id
        WHERE r.kind = 'textured_map'
        UNION
        SELECT c.id FROM gallery_images AS c
        JOIN chain ON c.parent_image_id = chain.id
    )
    SELECT id FROM chain
);

-- +goose Down
SELECT 1;
