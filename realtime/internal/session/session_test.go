package session

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestFinishSessionBuildsSoloRunContract(t *testing.T) {
	const anonID = "8f14e45f-ceea-467a-9f0a-1c3bd0a3f7d2"

	sess := NewSession(nil, 0, true, anonID, NewValidaator())
	sess.ID = "session-1"
	sess.StartSession("text-1", "solo", "abc")
	sess.StartTime = time.Date(2026, time.August, 13, 18, 42, 11, 418_000_000, time.UTC)

	for _, item := range []struct {
		char string
		t    int64
	}{
		{char: "a", t: 100},
		{char: "x", t: 105},
		{char: "Backspace", t: 120},
		{char: "b", t: 150},
		{char: "c", t: 210},
	} {
		if _, err := sess.ProcessKeystroke(item.char, item.t); err != nil {
			t.Fatalf("ProcessKeystroke(%q, %d): %v", item.char, item.t, err)
		}
	}

	_, run := sess.FinishSession()

	if run.SessionID != "session-1" || run.TextID != "text-1" || run.Mode != "solo" {
		t.Fatalf("unexpected run identity: %+v", run)
	}
	if run.UserID != nil {
		t.Fatalf("anonymous run has user_id=%d, want null", *run.UserID)
	}
	if run.AnonID == nil || *run.AnonID != anonID {
		t.Fatalf("anon_id=%v, want %q", run.AnonID, anonID)
	}
	if run.DurationMs != 110 || run.Typed != 4 || run.Correct != 3 || run.Errors != 1 {
		t.Fatalf("unexpected counters: duration=%d typed=%d correct=%d errors=%d", run.DurationMs, run.Typed, run.Correct, run.Errors)
	}
	if !reflect.DeepEqual(run.Flags, []string{"interval_below_minimum"}) {
		t.Fatalf("flags=%v", run.Flags)
	}
	if len(run.Keystrokes) != 5 || !run.Keystrokes[1].IsError || run.Keystrokes[2].Char != "Backspace" {
		t.Fatalf("unexpected keystrokes: %+v", run.Keystrokes)
	}

	encoded, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal run: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode run payload: %v", err)
	}
	for _, key := range []string{
		"session_id", "user_id", "anon_id", "text_id", "mode", "started_at",
		"duration_ms", "wpm_net", "wpm_raw", "accuracy", "typed", "correct",
		"errors", "flags", "keystrokes",
	} {
		if _, ok := payload[key]; !ok {
			t.Errorf("payload has no %q field: %s", key, encoded)
		}
	}
	for _, legacyKey := range []string{"wpm", "total_keystrokes", "is_personal_best"} {
		if _, ok := payload[legacyKey]; ok {
			t.Errorf("payload contains legacy field %q: %s", legacyKey, encoded)
		}
	}
}

func TestFinishSessionUsesNullableIdentityFields(t *testing.T) {
	sess := NewSession(nil, 42, false, "ignored-anon-id", NewValidaator())
	sess.StartSession("text-1", "solo", "a")

	_, run := sess.FinishSession()

	if run.UserID == nil || *run.UserID != 42 {
		t.Fatalf("user_id=%v, want 42", run.UserID)
	}
	if run.AnonID != nil {
		t.Fatalf("authenticated run has anon_id=%q, want null", *run.AnonID)
	}
	if run.Flags == nil || run.Keystrokes == nil {
		t.Fatalf("empty arrays must serialize as []: flags=%v keystrokes=%v", run.Flags, run.Keystrokes)
	}
}
