package gen

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// Validate answers the first rule a request breaks, in this order, with these sentences
// (the sentences only reach the log, but the order says which rule a request with two faults
// is refused for).
func TestValidateAnswersTheFirstBrokenRule(t *testing.T) {
	t.Parallel()
	img := &Image{MimeType: "image/png", Data: []byte("x")}
	edit := &Edit{Original: "o", Instruction: "make it darker", Previous: Image{MimeType: "image/png", Data: []byte("x")}}
	refs := func(objects, characters int) []Reference {
		var out []Reference
		for range objects {
			out = append(out, Reference{Image: Image{Data: []byte("x")}})
		}
		for range characters {
			out = append(out, Reference{Image: Image{Data: []byte("x")}, Character: true})
		}
		return out
	}
	const (
		layoutRule = "gen: a drawing goes with a layout, and a layout with a drawing, in a request that is not an edit"
		roomsRule  = "gen: rooms go only with the textured map, and at most MaxRooms of them"
	)
	tooManyRooms := make([]string, MaxRooms+1)
	tests := []struct {
		name string
		r    Request
		want string // "" for valid
	}{
		{"a prompt", Request{Prompt: "a cave", AspectRatio: "1:1"}, ""},
		{"an edit", Request{AspectRatio: "1:1", Edit: edit}, ""},
		{"a drawing with its layout", Request{Prompt: "a cave", AspectRatio: "1:1", Layout: LayoutScene, Drawing: img}, ""},
		{"the textured map with its rooms", Request{Prompt: "a cave", AspectRatio: "1:1", Layout: LayoutTexture, Drawing: img, Rooms: []string{"Sala 1"}}, ""},
		{"references up to the limits", Request{Prompt: "a cave", AspectRatio: "1:1", References: refs(MaxObjectReferences, MaxCharacterReferences)}, ""},
		{"a ratio that is not the model's", Request{Prompt: "a cave", AspectRatio: "7:5"}, `gen: aspect ratio "7:5" is not one the model returns`},
		{"no ratio, and no prompt", Request{}, `gen: aspect ratio "" is not one the model returns`},
		{"an empty prompt", Request{Prompt: "  \n", AspectRatio: "1:1"}, "gen: the prompt is empty"},
		{"an edit with no instruction", Request{AspectRatio: "1:1", Edit: &Edit{Instruction: " ", Previous: edit.Previous}}, "gen: an edit needs an instruction and the previous image"},
		{"an edit with no previous image", Request{AspectRatio: "1:1", Edit: &Edit{Instruction: "x"}}, "gen: an edit needs an instruction and the previous image"},
		{"a drawing with no layout", Request{Prompt: "a cave", AspectRatio: "1:1", Drawing: img}, layoutRule},
		{"a layout with no drawing", Request{Prompt: "a cave", AspectRatio: "1:1", Layout: LayoutScene}, layoutRule},
		{"a layout that does not exist", Request{Prompt: "a cave", AspectRatio: "1:1", Layout: "sphere", Drawing: img}, layoutRule},
		{"a layout in an edit", Request{AspectRatio: "1:1", Edit: edit, Layout: LayoutScene, Drawing: img}, layoutRule},
		{"rooms without the textured map", Request{Prompt: "a cave", AspectRatio: "1:1", Layout: LayoutScene, Drawing: img, Rooms: []string{"Sala 1"}}, roomsRule},
		{"rooms with no layout", Request{Prompt: "a cave", AspectRatio: "1:1", Rooms: []string{"Sala 1"}}, roomsRule},
		{"too many rooms", Request{Prompt: "a cave", AspectRatio: "1:1", Layout: LayoutTexture, Drawing: img, Rooms: tooManyRooms}, roomsRule},
		{"too many objects", Request{Prompt: "a cave", AspectRatio: "1:1", References: refs(MaxObjectReferences+1, 0)}, "gen: at most 10 objects and 4 characters as references"},
		{"too many characters", Request{Prompt: "a cave", AspectRatio: "1:1", References: refs(0, MaxCharacterReferences+1)}, "gen: at most 10 objects and 4 characters as references"},
		{"the ratio before the prompt", Request{AspectRatio: "7:5", Prompt: ""}, `gen: aspect ratio "7:5" is not one the model returns`},
		{"the prompt before the layout", Request{AspectRatio: "1:1", Layout: LayoutScene}, "gen: the prompt is empty"},
		{"the layout before the rooms", Request{Prompt: "p", AspectRatio: "1:1", Rooms: []string{"Sala 1"}}, roomsRule},
		{"the rooms before the references", Request{Prompt: "p", AspectRatio: "1:1", Layout: LayoutScene, Drawing: img, Rooms: []string{"a"}, References: refs(MaxObjectReferences+1, 0)}, roomsRule},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.r.Validate()
			switch {
			case tc.want == "" && err != nil:
				t.Errorf("Validate() = %v, want valid", err)
			case tc.want != "" && (err == nil || err.Error() != tc.want):
				t.Errorf("Validate() = %v, want %q", err, tc.want)
			}
		})
	}
}

// parseAnswer's errors, word for word: what the log reads and what each error is (errors.Is
// and As are what the module decides on).
func TestParseAnswerSentencesAreFrozen(t *testing.T) {
	t.Parallel()
	b64 := base64.StdEncoding.EncodeToString(onePixel)
	long := strings.Repeat("é", maxLoggedMessage) // 2 bytes each: cut in the middle of a character
	tests := []struct {
		name   string
		status int
		body   string
		want   string // the error's text
		is     error
	}{
		{"a 500 with no error object", 500, `{}`, "the image service is unavailable\nstatus 500", ErrUnavailable},
		{"a 400 with an error object and no message", 400, `{"error":{"code":"invalid_request"}}`, "the image service is unavailable\nstatus 400", ErrUnavailable},
		{"a message with the string code", 400, `{"error":{"message":"bad value","code":"invalid_request"}}`, "the image service is unavailable\nstatus 400: invalid_request: bad value", ErrUnavailable},
		{"a message with the numeric code", 400, `{"error":{"message":"bad value","code":400}}`, "the image service is unavailable\nstatus 400: 400: bad value", ErrUnavailable},
		{"the status word over the code", 429, `{"error":{"message":"slow down","code":429,"status":"RESOURCE_EXHAUSTED"}}`, "the image service is unavailable\nstatus 429: RESOURCE_EXHAUSTED: slow down", ErrUnavailable},
		{"the detail over the status word", 500, `{"error":{"message":"m","status":"INTERNAL","details":[{"reason":"WHY"}]}}`, "the image service is unavailable\nstatus 500: WHY: m", ErrUnavailable},
		{"an error object in a 200", 200, `{"error":{"message":"m","status":"S"}}`, "the image service is unavailable\nstatus 200: S: m", ErrUnavailable},
		{"the message is cut short", 500, `{"error":{"message":"` + long + `","status":"S"}}`, "the image service is unavailable\nstatus 500: S: " + strings.Repeat("é", maxLoggedMessage/2) + "...", ErrUnavailable},
		{"a refusal word needs a 400", 500, `{"error":{"message":"blocked for safety"}}`, "the image service is unavailable\nstatus 500: : blocked for safety", ErrUnavailable},
		{"safety is the first word", 400, `{"error":{"message":"safety and blocked"}}`, "the image service refused the request: safety", nil},
		{"prohibited", 400, `{"error":{"message":"PROHIBITED content"}}`, "the image service refused the request: prohibited", nil},
		{"blocked", 400, `{"error":{"message":"it was blocked"}}`, "the image service refused the request: blocked", nil},
		{"recitation", 400, `{"error":{"message":"Recitation"}}`, "the image service refused the request: policy", nil},
		{"policy", 400, `{"error":{"message":"against the policy"}}`, "the image service refused the request: policy", nil},
		{"a key refused beats a refusal word", 401, `{"error":{"message":"blocked","status":"UNAUTHENTICATED"}}`, "the image service did not accept the key: status 401, UNAUTHENTICATED", ErrNotAuthorized},
		{"a 403 with the code as reason", 403, `{"error":{"code":"denied"}}`, "the image service did not accept the key: status 403, denied", ErrNotAuthorized},
		{"a 401 with no error object", 401, `{}`, "the image service did not accept the key: status 401, ", ErrNotAuthorized},
		{"a safety stop in the errors", 200, `{"status":"failed","errors":[{"code":"X","message":"x"},{"code":"BLOCKED_REASON","message":"x"}]}`, "the image service refused the request: blocked", nil},
		{"a stop with no safety word", 200, `{"status":"failed","errors":[{"code":"X","message":"x"}]}`, "the image service did not return an image", ErrNoImage},
		{"a completed answer with a refusal word in the errors", 200, `{"status":"completed","errors":[{"code":"SAFETY"}],"steps":[` + step("model_output", imageBlock(b64)) + `]}`, "", nil},
		{"an image block with no data is skipped", 200, answer("completed", step("model_output", imageBlock(b64), `{"type":"image","data":""}`)), "", nil},
		{"data that is not base64", 200, answer("completed", step("model_output", `{"type":"image","data":"!!"}`)), "the image service did not return an image", ErrNoImage},
		{"data that decodes to nothing", 200, answer("completed", step("model_output", `{"type":"image","data":"="}`)), "the image service did not return an image", ErrNoImage},
		{"not JSON", 200, `<html>`, "the image service is unavailable\ncannot read the answer (status 200)", ErrUnavailable},
		{"not JSON, with an error status", 502, ``, "the image service is unavailable\ncannot read the answer (status 502)", ErrUnavailable},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			img, err := parseAnswer(tc.status, strings.NewReader(tc.body))
			if tc.want == "" {
				if err != nil || len(img.Data) == 0 {
					t.Errorf("parseAnswer() = %v, %v; want an image", img, err)
				}
				return
			}
			if err == nil || err.Error() != tc.want {
				t.Errorf("error = %q, want %q", err, tc.want)
			}
			if tc.is != nil && !errors.Is(err, tc.is) {
				t.Errorf("error = %v, want errors.Is %v", err, tc.is)
			}
		})
	}
}

// The picture an answer gives back: the last image of the model's output steps, in the mime type
// it came with, or JPEG when it came with none (what the request asked for).
func TestParseAnswerImageIsFrozen(t *testing.T) {
	t.Parallel()
	b64 := base64.StdEncoding.EncodeToString(onePixel)
	other := base64.StdEncoding.EncodeToString([]byte("other"))
	for name, tc := range map[string]struct {
		body string
		mime string
		data string
	}{
		"its own mime type":          {answer("completed", step("model_output", `{"type":"image","mime_type":"image/png","data":"`+b64+`"}`)), "image/png", string(onePixel)},
		"no mime type is JPEG":       {answer("completed", step("model_output", `{"type":"image","data":"`+b64+`"}`)), "image/jpeg", string(onePixel)},
		"the last output step wins":  {answer("completed", step("model_output", imageBlock(other))+","+step("model_output", imageBlock(b64))), "image/jpeg", string(onePixel)},
		"a later thought is skipped": {answer("completed", step("model_output", imageBlock(b64))+","+step("thought", imageBlock(other))), "image/jpeg", string(onePixel)},
		"text blocks are skipped":    {answer("completed", step("model_output", imageBlock(b64), `{"type":"text","text":"x","data":"`+other+`"}`)), "image/jpeg", string(onePixel)},
	} {
		img, err := parseAnswer(200, strings.NewReader(tc.body))
		if err != nil || img.MimeType != tc.mime || string(img.Data) != tc.data {
			t.Errorf("%s: parseAnswer() = %q %q, %v; want %q %q", name, img.MimeType, img.Data, err, tc.mime, tc.data)
		}
	}
}
