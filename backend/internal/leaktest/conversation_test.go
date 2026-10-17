package leaktest

import (
	"net/http"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The scene's conversation (MR-051, RN-36): a line is for the whole table or for chosen readers, and
// whoever is not a reader never learns the text, the picture, that it exists, or the audio. The lines
// are markers: the reads, the stream and the summary are scanned for them like for any secret, and the
// readers (and the master, who reads everything) are the positive control.

// conversationLine is a line whose audio is asked for, and who may hear it.
type conversationLine struct {
	id      string
	readers []*person
}

const (
	speakNarrator = playv1.SessionMessageSpeakerKind_SESSION_MESSAGE_SPEAKER_KIND_NARRATOR
	speakNPC      = playv1.SessionMessageSpeakerKind_SESSION_MESSAGE_SPEAKER_KIND_NPC
	speakOOC      = playv1.SessionMessageSpeakerKind_SESSION_MESSAGE_SPEAKER_KIND_OUT_OF_CHARACTER
)

// say sends a line as p and returns it as the sender reads it.
func (w *world) say(p *person, kind playv1.SessionMessageSpeakerKind, text string, edit func(*playv1.SendSessionMessageRequest)) *playv1.SessionMessage {
	req := &playv1.SendSessionMessageRequest{CampaignId: w.campaign, SpeakerKind: kind, Text: text, IdempotencyKey: newKey()}
	if edit != nil {
		edit(req)
	}
	return must(p.play.SendSessionMessage(w.t.Context(), rq(req))).GetMessage()
}

// buildConversation has the table talk: a line for everyone, a line for Ana alone with a picture, an
// NPC's line for Caio alone, a player's line for the master alone, and a player's line for everyone.
func (w *world) buildConversation() {
	m := w.master
	w.imgLine = w.image("line-image", 70, 50, w.ana)
	w.secrets.id("line-image", w.imgLine, w.ana)

	all := w.say(m, speakNarrator, w.secrets.public("line-table"), nil)
	w.lines = append(w.lines, conversationLine{all.GetId(), []*person{w.ana, w.caio}})
	toAna := w.say(m, speakNarrator, w.secrets.marker("line-ana", w.ana), func(r *playv1.SendSessionMessageRequest) {
		r.ReaderUserIds, r.ImageId = []string{w.ana.id}, w.imgLine
	})
	w.lines = append(w.lines, conversationLine{toAna.GetId(), []*person{w.ana}})
	toCaio := w.say(m, speakNPC, w.secrets.marker("line-caio", w.caio), func(r *playv1.SendSessionMessageRequest) {
		r.SpeakerId, r.ReaderUserIds = w.merchant.GetId(), []string{w.caio.id}
	})
	w.lines = append(w.lines, conversationLine{toCaio.GetId(), []*person{w.caio}})
	w.say(w.caio, speakOOC, w.secrets.marker("line-caio-whisper", w.caio), func(r *playv1.SendSessionMessageRequest) { r.OnlyMaster = true })
	w.say(w.ana, speakOOC, w.secrets.public("line-ana-out"), nil)
}

// checkLineAudio asks for the audio of each line as everyone: the master and the readers hear it, and
// everyone else gets the 404 of a line that does not exist (RN-36).
func checkLineAudio(t *testing.T, w *world) {
	t.Helper()
	everyone := append(w.players(), w.anonymous())
	for _, line := range w.lines {
		path := "/session-messages/" + line.id + "/audio"
		if status, body := w.master.get(path); status != http.StatusOK {
			t.Errorf("line %s: the master got HTTP %d %s, want 200 (the positive control)", line.id, status, shorten(body))
		}
		for _, p := range everyone {
			want := http.StatusNotFound
			if slicesContains(line.readers, p) {
				want = http.StatusOK
			}
			status, body := p.get(path)
			switch {
			case want == http.StatusOK && status != http.StatusOK:
				t.Errorf("line %s: %s got HTTP %d, want 200: the line is theirs (the control)", line.id, p.name, status)
			case want == http.StatusNotFound && status != http.StatusNotFound && status != http.StatusUnauthorized:
				t.Errorf("line %s: %s got HTTP %d %s, want 404 like a line that does not exist (RN-36)", line.id, p.name, status, shorten(body))
			}
			for _, f := range w.inspect(p, reply{status: status, body: body}, nil) {
				t.Errorf("line %s: %s: %s", line.id, p.name, f)
			}
		}
	}
	if status, _ := w.ana.get("/session-messages/" + newKey() + "/audio"); status != http.StatusNotFound {
		t.Errorf("a line that does not exist: HTTP %d, want 404", status)
	}
}
