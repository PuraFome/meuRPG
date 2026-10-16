# AI-generated images

The master asks a model (the Gemini API) for an image, and the server stores it in the gallery like any image: hidden from the players until the master shows it (MR-039, RN-28, ADR-0019).
The code is in `backend/internal/maps` (`imagegen.go`, `imagegen_map.go`; contract `ImageGenerationService` in `proto/meurpg/maps/v1/imagegen.proto`) and in `backend/internal/maps/images/gen`, a small package that only talks to the model and knows no campaign, person or database.
Configuration (`GEMINI_API_KEY`, `GEMINI_IMAGE_MODEL`, `IMAGE_GENERATOR`, the monthly and daily limits) is in [CONTRIBUTING](../../development.md#ai-generated-images); costs and memory are in [Operations](../../operations/reference.md#generated-images-the-gemini-api).

| `ImageGenerationService` call | What it does |
| --- | --- |
| `GetImageGenerationStatus` | Whether generation is on and the month's tally ("Restam 17 de 20 imagens em outubro"): the limit, the spend, when it restarts (the first instant of the next month, Brasília time) and a request's limits (500 characters, 10 object images, 4 character images). Works with generation off |
| `GenerateSceneImage` | The scene art: the master's text, the style, the aspect ratio (one of 10, default 16:9) and gallery images as references. Answers at once with the `PENDING` request |
| `EditGeneratedImage` | An adjustment of a generated image: a new request that spends a slot |
| `GetImageGeneration` | The request's state (`PENDING`, `DONE` with the image, `REFUSED` or `FAILED` with a Portuguese sentence, `CANCELED`). With `wait_seconds` (up to 25) it is a **long poll** ([below](#shutdown-and-the-long-poll)). There is no stream hint: a player never knows a request exists |
| `CancelImageGeneration` | Before the request leaves the server, returns the slot; after, only stops the wait |
| `ListImageEdits` | An image's chain of adjustments, oldest to newest, each with the text that made it |
| `GetMapImageReference`, `GenerateMapImage`, `UseGeneratedImageAsMapImage` | The map-based modes ([From a map](#images-made-from-a-map)) |

Only the master calls.
A player, a pending member and an outsider get `not_found`, as for a hidden image (the service does not reveal that the campaign generates images); without a session, `unauthenticated`.
The refusals the master can resolve are `failed_precondition` with the `ImageGenerationBlocked` detail: `OFF` (no key, or no image storage), `LIMIT_REACHED` (with the month's tally), `DAILY_LIMIT_REACHED` (the server-wide cap, see Abuse limits in [Architecture](../api.md#abuse-limits)), `GALLERY_FULL`, `REQUEST_TOO_LARGE`, `PLAYERS_SEE_NOTHING`, `MAP_HAS_NO_GRID`, `MAP_IMAGE_TOO_LARGE` and `MAP_CHANGED`.

## The month's slot, in three steps (RN-28)

1. **Reserve.** A short transaction (`reserve`) returns the request of an already-used key, or checks that generation is on, the month's tally, the gallery and the reference images (all the campaign's, read through the transaction), then inserts the `image_requests` row. The row is the slot: the month counts the rows not given back. `SERIALIZABLE` makes two requests for the last slot retry, and the second sees it taken.
2. **Call the model, outside any transaction.** One goroutine per request (`run`; one model call at a time on the server) takes the already-prepared images, sets `sent_at` (if a "Cancelar" came first, nothing goes out) and calls the `Generator`. It derives from a service context (cancelled on shutdown) with a 6-minute timeout (`generationTimeout`), not from the request's context: the master may close the page.
3. **Store or give back.** The image goes through the same path as an upload (`process`, `storeImage`: decoded and re-encoded without metadata, with the gallery quota) and becomes a `generated` image. The gallery row and the request's end go in the **same transaction**, allowed once only. The name comes from the request (the scene art, the map's isometric view or the adjusted image's name; "Imagem n" only when there is none). If an adjustment's parent was deleted meanwhile, the child is left without a parent and the slot stays spent.

What happens to the slot:

| Outcome | Request ends | Slot |
| --- | --- | --- |
| The model certainly was not billed (a refusal, a response without an image, a key refused by Google) | `refused` or `failed` | Returns |
| The call may have been billed (a timeout, a connection cut after the request left, an unreadable answer) | `failed` | **Stays spent** |
| The service returned an image the server then cannot store (the gallery filled up, or the answer cannot be decoded) | `failed` | **Stays spent** (`FinishImageRequestSpent`) |
| "Cancelar" after the request left | `CANCELED`, with `image_id` if the image arrives | Stays spent; only the wait stops, and an image that comes still goes to the gallery |

A key refused by Google (401 or 403, an operator's error) becomes "O serviço de imagens não está disponível", with an error log line (Google's status and reason, never the key).
Every failure message is a Portuguese sentence we wrote, never Google's.

Each request carries a repeat-guard key (`idempotency_key`, unique per campaign): repeating the call returns the first request and never generates again.

**Lost requests.** A `PENDING` request that never left the server for 10 minutes (`staleAfter`) fails and returns the slot. One that left and stays `PENDING` for 10 minutes fails as `timeout` with the slot **spent**; an image that still arrives enters the gallery, once only. This is settled when someone reads the request, the month's state, or asks for another image of the campaign (so "Restam" is right). The reservation counts requests in flight against the gallery's space, so a full gallery does not fail after Google bills.

## Shutdown and the long poll

- **Shutdown.** `cmd/api` cancels the service context when shutdown begins (`OnShutdown`): a request still queued returns its slot at once, long polls answer, and a call already sent is cut and follows the rule above. After the HTTP server stops, the goroutines are waited for about 1 s (inside Cloud Run's 10 s) before the pool closes.
- **The long poll and Cloud Run.** Cloud Run gives CPU only during a request, so a goroutine working after the response may be left without CPU. The app therefore keeps **one long poll open** (`GetImageGeneration` with `wait_seconds` 25, again and again) while it waits, and the instance always has a request in progress during generation. The wait uses an in-memory notifier per request (single instance): the goroutine notifies on finishing, the call rereads the row before answering, and the end of the wait also answers. With more than one instance, the wait still ends at its deadline.

## Memory

Every live request keeps its shrunk references in memory until its call goes out, also while it waits for the call slot.
So at most **5 requests are alive at once** (`maxPendingRequests`: one calling the model, four waiting).
A sixth is `resource_exhausted` ("Muitas imagens sendo geradas agora. Espere uma terminar e tente de novo."), before any image is read and without spending the month's slot.
The measured worst case and how to measure it again are in [Operations](../../operations/reference.md#generated-images-the-gemini-api).

- Each reference image is its **reference**: a JPEG of at most 1024 px on the longer side, quality 85, stored next to the image in the blob store (`<id>.ref`, only for an image larger than 1024 px). It is made on upload, from the image already decoded in `images.Process`; an older image gets it the first time it is used, inside the upload slot, shrunk in strips. Deleting the image deletes the reference. All of this happens **before** the month's slot is reserved.
- The whole request body is at most 8 MiB (with base64). Above that after shrinking, `REQUEST_TOO_LARGE`, no slot spent.
- The body is written as a stream (`io.Pipe`, base64 while writing), never built whole.
- The response is at most 8 MiB and is read straight into a typed structure.

## What goes to Google

The master's text as written, the style (one English word, "watercolor"), the aspect ratio and the gallery images they chose.
On an adjustment: the previous image, the first image's text, the adjustments already made and the new one.
The server adds no name, e-mail, sheet or ID: `gen.Request` has no field where they would fit, and `TestRN28_NothingPersonalGoesToTheModel` reads the body that would go out (through the fake) and checks it.
The master's text never goes to a log; error messages state a rule, never the text.
The screen warns what will go.
The full list, including the map modes, is in [Privacy](../../privacy.md#what-generated-images-send-to-google-mr-039-rn-28).

## The `Generator`

An interface of two methods (`Generate(ctx, Request) (Image, error)` and `Model()`) with two implementations.

**`gen.Gemini`** uses `net/http` and no SDK.

- It calls the **Interactions API** (`POST https://generativelanguage.googleapis.com/v1beta/interactions`), not `generateContent`, because only it has `store`: `store=false` keeps the call from being stored as an interaction. Under the paid services' terms Google may still keep the data briefly or in cache, in any country ([Privacy](../../privacy.md#what-generated-images-send-to-google-mr-039-rn-28)).
- The body is `{"model", "input": [{"type":"text","text"}, {"type":"image","mime_type","data"}...], "response_format": {"type":"image","mime_type":"image/jpeg","aspect_ratio","image_size":"1K"}, "store": false}` in `snake_case`. The API only answers JPEG (it refuses `image/png` here), and everything downstream sniffs the type.
- The key goes only in the `x-goog-api-key` header, as a `config.Secret` (`[REDACTED]` in any formatting).
- The header `Api-Revision` (`APIRevision`, in `gemini.go`) pins the response to the `steps` format. The response only counts with `status` `completed`; the image is the **last** `image` block of the `model_output` steps, and `thought` steps are ignored. A draft alone, or an `incomplete`, is "O serviço não gerou uma imagem".
- Only a `429` or a `5xx` response retries, once, after 2 s. A timeout, a connection cut after sending and an unreadable response are **never** retried (they may have been billed). The one replay left to `net/http` is a reused keep-alive connection that failed before any byte was written (nothing reached Google); the request has a `GetBody` for that. No redirect is followed, so the key header never goes along.
- A safety stop (`errors` with `SAFETY` or `PROHIBITED`, or a `400` saying "blocked") becomes `RefusedError`, without Google's message. A `401` or `403` becomes `NotAuthorizedError`. Any other `4xx` is `ErrUnavailable` whose text carries Google's message (cut at 300 characters) for the log only. The error body is read in both shapes Google uses (a string `code` and a numeric one).

**`gen.Fake`** serves the local environment, CI and tests (`IMAGE_GENERATOR=fake`, refused on Cloud Run). It is deterministic: the same text gives the same bytes, a valid PNG of 256 px on the longer side in the requested aspect ratio. A `[recusa]`, `[vazio]`, `[erro]` or `[lento]` in the text makes it refuse, return no image, fail or be slow; a `Hook` lets a test hold the request in the air, and `Calls()` stores what arrived, with the JSON body Gemini would have received.

References: [image generation](https://ai.google.dev/gemini-api/docs/image-generation) (the body, the 10 aspect ratios, up to 10 object images and 4 character images, `512px` to `4K`), [Interactions API](https://ai.google.dev/gemini-api/docs/interactions) (`store`, retention), [pricing](https://ai.google.dev/gemini-api/docs/pricing) and the [May 2026 breaking-changes guide](https://ai.google.dev/gemini-api/docs/interactions-breaking-changes-may-2026) (`Api-Revision`).
The real REST response is only confirmed with a key: the optional `TestRealGemini` (`MEURPG_TEST_GEMINI_API_KEY`) exists for that.

## The adjustment chain

`gallery_images.parent_image_id` links an adjustment to the image it came from.
The model keeps no state: each adjustment sends again the previous image, the chain's first image's text, the adjustments already made and the new one (the server reads the chain), with the same aspect ratio and style as the source and `store=false`.
`ListImageEdits` returns the image's whole tree with each one's text.

## Images made from a map

The master can ask for a scene's art, the isometric view or the textured map itself from a map with a grid.
It is the same service and path (`reserve`, `run`, `process`, `storeImage`, the tally, the long poll, cancel); what changes is the **reference drawing** sent to the model and what is done with the result.
Pure helpers: `maps/refimg` (the drawing and the aspect-ratio arithmetic) and `maps/images` (`CropFit`).

| Call | What it does |
| --- | --- |
| `GetMapImageReference` | What a request would send, before asking and without spending a slot: the drawing as a PNG up to 512 px, the squares seen ("12 de 600"), whether the players see anything, the textured map's aspect ratio and **the NPCs the master may mark** ("Quem aparece na imagem": id, name, portrait) |
| `GenerateMapImage` | `MAP_SCENE`, `ISOMETRIC` or `TEXTURED_MAP` of a map, with text, style, aspect ratio (not for the textured map), gallery references and the NPCs that appear. Answers at once with the `PENDING` request |
| `UseGeneratedImageAsMapImage` | "Usar como imagem do mapa" for a ready textured map: swaps the map's image for it and returns the map |

`GenerateMapImage` is `begin` with one more step in `prepare` (`prepareMap`) before the slot is reserved: it reads the map and layers, computes the view, checks the NPCs, draws the reference and stores what matters at the time of use (the map's image, columns, grid factor, size in pixels) in `image_requests` columns.
A refusal (no grid, nobody on the map, an NPC they do not see) spends no slot.
The reference goes as the first image after the previous one of an adjustment and before the gallery references.
The response goes through `process` (scene art, isometric view) or `cropToMap` (textured map), inside the same image-processing slot.

### The players' view

The scene art and the isometric view start from what the players see **now** (`playersSeenOf`, ADR-0019).

- **With fog on,** the view is the **union** of the players' living characters' sight: the same arithmetic as the fog (`newSight`) with group sight forced and **without memory**. The seen squares include the wall next to a seen square. With no character on the map, or characters who see no square (magical darkness, blind), the view is empty and the request is refused (`PLAYERS_SEE_NOTHING`, no slot, `players_see_something` false).
- **Without fog,** the view is the whole map minus what is hidden: never a hidden token, point, trap, treasure or door.
- **Creatures** are the players' characters (a blue disc) and the NPCs that are not hidden, on a square seen now (a red disc). While a combat runs on the map no NPC enters (combatants are not map tokens); the players' characters stay on the combatant's square. The text tells the model to draw a creature only on a disc or where the master's own words ask for one (`creaturesRule`).
- **Floor and walls** come from the layers (`floorPlan`): the painted wall is solid, **a still-hidden secret door is a wall** (RN-10, RN-26) and any other door is floor (the app draws doors as an overlay). In a generated dungeon the rock behind the walls is also solid (`solidOf`, the same drawing as "Redesenhar").
- **The map's uploaded image never enters the drawing:** it may show a secret door or a hidden enemy painted, and the server cannot know. The drawing is only floor and walls in the app's paper-sheet style (`dungeonimg`), cropped to the rectangle of seen squares plus one square of margin, with **black where nobody sees**.

### The NPCs that appear

For scene art and the isometric view the master marks up to 4 NPCs; their portraits enter as character references, along with the character images picked in the gallery (a repeated image counts once).
The server **only accepts NPCs the players see** (`invalid_argument` otherwise) and **refuses in `character_image_ids` the portrait of an NPC that has a token on this map and that the players do not see now** (`checkCharacters`).
A gallery image that is not an NPC's portrait stays the master's choice.
`checkCharacters` also refuses a hidden NPC's portrait in `object_image_ids`.
Each error carries `ImageGenerationInvalidField` with the field name, and the refusal comes before the slot is reserved.
An NPC without a portrait sends no reference.
Portraits are read through `CharacterDirectory.NpcPortraits` (module `characters`).

### The textured map

The textured map starts from the whole map, because fog already hides it from each player, and **carries no creature**.

- `npc_character_ids` and `character_image_ids` must be empty (`invalid_argument` with `ImageGenerationInvalidField`, before the slot). `object_image_ids` may not hold the portrait of any NPC with a token on the map, seen or not (`checkTexturePortraits`). `GetMapImageReference` returns no NPCs, and the players' view is not computed. The image may become the map the players read, and a creature painted on it is not a map creature.
- **The drawing** (`refimg.RenderPadded`) is the whole map with the secret door as a wall, drawn **flat** (floor in paper colour with no ink outline; walls, rock and padding in one nearly black, `refimg.flatPalette`) and completed with rock up to **the model aspect ratio closest to the map's** (`refimg.Closest`, of the 10 ratios; the request's own aspect ratio is ignored). The request stores **where the map sat in the drawing** (the fractions of the map's rectangle on the canvas, `pad_x0` to `pad_y1`).
- **The result** is cropped back with those fractions (`refimg.CropBy`) and **fitted to the map image's size** (`images.CropFit`: Catmull-Rom in strips, JPEG without metadata). It has exactly the map image's size, so it can take its place with the same grid.
- **The limit.** The map's image can be up to **16 megapixels** (for example 4,000 × 4,000 px, or 8,000 × 2,000), because the crop's working copy has 4 bytes per pixel. Above that, `MAP_IMAGE_TOO_LARGE` (no slot); `GetMapImageReference` warns beforehand (`texture_too_large`, `max_texture_pixels`) so the screen turns the choice off. `CropFit` also refuses (`ErrDimensions`, and the slot returns) a model answer over 4,096 px on a side, or one whose decoding with the output's working copy would exceed 178 MiB (`maxFitBytes`).
- **The text** (`gen.textureIntro`) states the scale (a square is 1,5 m / 5 ft; a one-square corridor is a walkable passage) and what each kind of square may hold: light squares are the only walkable floor, painted flat, nothing raised on them, keeping their exact shape and width; dark squares are solid rock, painted as the top of thick walls or bedrock, with wall faces and shadows on the dark side of each boundary; light from straight above, no perspective. The room list of a generated dungeon (`Sala 3: 7 x 5 squares`, name and size only) goes only in this mode and only when the map is a generated dungeon.
- **An adjustment** of a textured map inherits the source request's base (the map, image, grid, size, walls hash) and asks the model for the aspect ratio closest to the map image's. The response is center-cropped to the map's aspect ratio (`refimg.CenterCrop`) and then fitted to its size (`fitToMap`): never stretched, so `UseGeneratedImageAsMapImage` accepts it with the same checks.

### "Usar como imagem do mapa"

`UseGeneratedImageAsMapImage` is the narrow path `replaceMapImageKeepingLayers` shared with "Redesenhar" of a generated dungeon ([Dungeons](dungeons.md#redrawing-keeps-the-layers)).
In one transaction with the map row locked, the map's image becomes the generated one **without touching the layers, points, tokens or what the players remember** (the pixel size and the grid are the same).

- **It checks first that the map is still the one the request started from:** the same image, columns, grid factor, size **and walls** (the request stores a hash of the floor and walls the drawing showed, `map_plan_hash`, with the secret door as a wall; painting a wall or revealing a secret door changes it, opening or locking a door does not). The image counts as the same when it is the map's image at request time, or the picture "Usar" put on the map from this request or an earlier edit of its chain. Otherwise: `ImageGenerationBlocked` `MAP_CHANGED` ("gere de novo").
- **On a fogged map the map's image is always its alone** (as in `UpdateMap`; `prepareFogCopy`, `prepareReuseCopy`). If the generated image is also shown to the players, is an NPC's portrait, another map's background or on the stage, the map gets a **copy** (read before the transaction, the row inside it, the files deleted if unused), so a player never reaches the map's image ([Fog](fog.md#the-image-of-a-fogged-map)).
- **Repeating.** The request stores the image "Usar" put on the map (`used_map_image_id`, in the same transaction). A repeated "Usar" with it still the map's image answers the same without a new copy or write; if the map's image changed since, `MAP_CHANGED`. The old image stays in the gallery.
- **Afterwards** the fog tiles are rebuilt and the map changes for those who read it (`publishMapChanged`). On a generated dungeon, "Redesenhar" answers `IMAGE_CHANGED` from then on (the image is no longer the generator's), and a "Usar" after, or racing with, a "Redesenhar" is refused (the locked map row decides who is first).

### Kind, name and key

- `gallery_images.generated_kind` stores a generated image's mode (`scene`, `map_scene`, `isometric`, `textured_map`; empty on an upload), and an adjustment inherits it. `GalleryImage.shows_whole_map` tells the master it is a `textured_map`.
- `image_requests.image_name` is the name the image will have: the master's, or a default that **never comes from the request's text nor from a hidden map's name** (a player to whom the image is shown reads the name, RN-10). The default is the map and mode when the players see the map ("Masmorra de Mirathel · vista isométrica"), otherwise the mode and the day ("Arte da cena · 06/10"), or the adjusted image's name with " (ajuste)". The screen fills the field with the same rule (a scene point's name only if revealed).
- The repeat-guard key is the app's, per attempt: it keeps it while nothing came back and the form did not change, and repeats a lost response (`unavailable`) once with the same key.

### What goes to Google, by mode

The master's text, the style, the aspect ratio, the gallery references, the marked NPCs' portraits (scene art and isometric view only) and the drawing: the floor and walls of the seen squares in the black of the rest with a disc per seen creature (scene art, isometric view), or the whole map (textured map), plus the room list only for the textured map of a generated dungeon.
Never a person's, character's or NPC's name, e-mail, sheet, ID or map text.
`TestMR039_TheSceneArtAndTheIsometricViewOfAMap` reads the body that would go out and checks it.

## The client

`web/src/app/core/images` holds what is not screen.

- `imagegen-client.ts`: the `ImageGenerationService`.
- `imagegen-form.ts`: the form becomes the request (the textured map sends no creature or aspect ratio).
- `imagegen-errors.ts`: each refusal by the typed detail, never the message.
- `imagegen-run.ts` (`ImageRun`): asks, keeps **one** 25 s long poll open until the request leaves `PENDING`, and cancels. A "Cancelar" pressed before the server answers cancels the request it made when the response arrives. The app does not see the instant the request leaves the server; the cancellation response's `slot_spent` says what happened to the slot.

"Mostrar aos jogadores" uses the session's `SetShownImage` (it needs an open session).
The screens are in `shared/image-generate`; the browser never generates or draws the image.
