-- +goose Up
-- image_requests is one request to generate or edit an image with an image
-- model (MR-039, RN-28, ADR-0019), and the monthly count of a campaign's
-- generated images: the rows of a month that were not refunded are the slots
-- it spent.
--
-- The life of a row:
--   - pending, sent_at NULL: the slot is reserved and the request has not left
--     the server. Cancel here refunds the slot.
--   - pending, sent_at set: the request went to the model. Cancel only stops
--     the wait (status canceled); the slot stays spent, and an image that still
--     arrives goes to the gallery (image_id).
--   - done: the image is in the gallery (image_id).
--   - refused: the service refused the prompt or stopped for safety (reason).
--   - failed: no image came, or the service did not answer (reason).
--   A refused or failed request is refunded (refunded = true): nothing was
--   generated, so it does not count.
--
-- idempotency_key comes from the client: asking twice with the same key in a
-- campaign gives the same row (a unique index, in 00192).
--
-- prompt is the master's own text (their data, see docs/privacidade.md): it is
-- what was sent to the model, and for an edit, the new instruction. The
-- server adds nothing personal to it. style and aspect_ratio are the closed
-- values ImageGenerationService validates. reference_ids and character_ids are
-- the gallery images sent along as objects and as characters (as text, the IDs
-- the master chose). source_image_id is
-- the image an edit started from. number is the "Imagem n" the gallery shows:
-- the order of the campaign's requests. quota_month is the month the slot
-- counts in, "YYYY-MM" in Brazil's fixed UTC-3.
--
-- requested_by SET NULL when the account goes; image_id and source_image_id
-- SET NULL when the image is deleted; the campaign's rows go with it.
CREATE TABLE IF NOT EXISTS image_requests (
    id UUID PRIMARY KEY,
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    requested_by UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    idempotency_key TEXT NOT NULL,
    kind TEXT NOT NULL,
    prompt TEXT NOT NULL,
    style TEXT NOT NULL,
    aspect_ratio TEXT NOT NULL,
    model TEXT NOT NULL,
    reference_ids TEXT[] NOT NULL,
    character_ids TEXT[] NOT NULL,
    source_image_id UUID NULL REFERENCES gallery_images (id) ON DELETE SET NULL,
    number INT4 NOT NULL,
    quota_month TEXT NOT NULL,
    status TEXT NOT NULL,
    reason TEXT NOT NULL,
    refunded BOOL NOT NULL,
    image_id UUID NULL REFERENCES gallery_images (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL,
    sent_at TIMESTAMPTZ NULL,
    finished_at TIMESTAMPTZ NULL,
    CONSTRAINT image_requests_kind_valid CHECK (kind IN ('scene', 'edit')),
    CONSTRAINT image_requests_status_valid CHECK (status IN ('pending', 'done', 'refused', 'failed', 'canceled')),
    CONSTRAINT image_requests_prompt_length CHECK (char_length(prompt) BETWEEN 1 AND 500),
    CONSTRAINT image_requests_key_length CHECK (char_length(idempotency_key) BETWEEN 1 AND 64)
);

-- +goose Down
DROP TABLE IF EXISTS image_requests;
