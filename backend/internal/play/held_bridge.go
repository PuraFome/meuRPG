package play

// While the paths move onto the gate one at a time (E4-2), the old answers also carry their answer in the
// form the gate reads, so that a path on either side works. Removed with forcedRoll and forcedOutside.

// replyOfOutside is the answer of AnswerOutsideInspiration and AnswerOutsideLucky as the gate reads it.
func replyOfOutside(f *forcedOutside) heldReply {
	a := heldReply{holdID: f.holdID, key: f.answerKey, question: askDie}
	if f.luckyKey != "" {
		a.key, a.question, a.reroll = f.luckyKey, askLucky, f.lucky
	}
	if f.use {
		a.die = &bonusUse{Face: f.face}
	}
	return a
}
