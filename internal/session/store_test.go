package session_test

import (
	"testing"
	"time"

	"glyph-protocol/internal/session"
)

func TestStageAndGetSlot(t *testing.T) {
	store := session.NewStore()
	sess, err := store.CreateSession(session.PermView | session.PermWriteDraft)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	slot, err := store.StageUserText(sess.ID, 0, []byte("hello world"), now)
	if err != nil {
		t.Fatal(err)
	}
	if slot.Version != 1 || string(slot.Content) != "hello world" {
		t.Fatalf("unexpected slot: %+v", slot)
	}
	got, err := store.GetUsableSlot(sess.ID, 0, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Content) != "hello world" {
		t.Fatal("content mismatch")
	}
}

func TestSlotExpiration(t *testing.T) {
	store := session.NewStore()
	sess, _ := store.CreateSession(session.PermView)
	now := time.Now()
	_, err := store.StageUserText(sess.ID, 1, []byte("expire me"), now)
	if err != nil {
		t.Fatal(err)
	}
	// Advance past TTL
	later := now.Add(61 * time.Second)
	_, err = store.GetUsableSlot(sess.ID, 1, later, false)
	if err == nil {
		t.Fatal("expected expiration error")
	}
}

func TestRateLimit(t *testing.T) {
	store := session.NewStore()
	sess, _ := store.CreateSession(session.PermView)
	now := time.Now()
	for i := 0; i < 10; i++ {
		if !store.CheckRateLimit(sess.ID, now) {
			t.Fatalf("unexpected rate limit at %d", i)
		}
	}
	if store.CheckRateLimit(sess.ID, now) {
		t.Fatal("expected rate limited on 11th")
	}
}
