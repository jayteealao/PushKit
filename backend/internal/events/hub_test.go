package events

import (
	"testing"
	"time"
)

// testWait bounds how long a test waits before declaring a blocked Publish a
// failure. The success path resolves immediately (Publish delivers synchronously),
// so this is only the failure deadline — never a synchronisation sleep.
const testWait = 500 * time.Millisecond

// TestHub_PerUserScoping is the #1 security gate: an event published for one user
// must reach only that user's subscribers and never leak to another user.
func TestHub_PerUserScoping(t *testing.T) {
	hub := NewHub()

	chA, unsubA := hub.Subscribe("userA")
	defer unsubA()
	chB, unsubB := hub.Subscribe("userB")
	defer unsubB()

	hub.Publish("userA", Event{Type: "file.uploaded", FileID: "f1"})

	// Publish is synchronous, so by the time it returns the event is already in
	// userA's buffer and would already be in userB's buffer if the boundary were
	// broken. Non-blocking receives therefore assert both facts deterministically.
	select {
	case got := <-chA:
		if got.FileID != "f1" {
			t.Fatalf("userA received %+v, want FileID f1", got)
		}
	default:
		t.Fatal("userA did not receive the event")
	}

	select {
	case leaked := <-chB:
		t.Fatalf("cross-user leak: userB received %+v", leaked)
	default:
		// expected — userB receives nothing
	}
}

// TestHub_DropNotBlock proves a full subscriber buffer never blocks Publish.
func TestHub_DropNotBlock(t *testing.T) {
	hub := NewHub()

	// A subscriber that is never drained; its buffer fills and overflows.
	_, unsub := hub.Subscribe("user")
	defer unsub()

	total := subscriberBuffer + 8

	done := make(chan struct{})
	go func() {
		for i := 0; i < total; i++ {
			hub.Publish("user", Event{Type: "file.uploaded", FileID: "f"})
		}
		close(done)
	}()

	select {
	case <-done:
		// Publish returned for every event despite the full buffer.
	case <-time.After(testWait):
		t.Fatal("Publish blocked on a full buffer — drop-not-block violated")
	}
}

// TestHub_DropNotBlock_OtherSubscriberUnaffected confirms that a slow consumer
// does not starve a healthy one: the drained subscriber still receives events
// up to its buffer while the undrained one silently drops the overflow.
func TestHub_DropNotBlock_OtherSubscriberUnaffected(t *testing.T) {
	hub := NewHub()

	_, unsubSlow := hub.Subscribe("user") // never drained
	defer unsubSlow()
	chFast, unsubFast := hub.Subscribe("user")
	defer unsubFast()

	total := subscriberBuffer + 8
	for i := 0; i < total; i++ {
		hub.Publish("user", Event{Type: "file.uploaded", FileID: "f"})
	}

	got := 0
	for {
		select {
		case <-chFast:
			got++
			continue
		default:
		}
		break
	}

	if got == 0 {
		t.Fatal("healthy subscriber received nothing")
	}
	if got > subscriberBuffer {
		t.Fatalf("healthy subscriber received %d events, exceeds buffer %d", got, subscriberBuffer)
	}
}

// TestHub_UnsubscribeLeakFree is the leak gate: subscribe raises the count,
// unsubscribe returns it to zero, and a double unsubscribe is a safe no-op.
func TestHub_UnsubscribeLeakFree(t *testing.T) {
	hub := NewHub()

	_, unsub := hub.Subscribe("user")
	if n := hub.subscriberCount("user"); n != 1 {
		t.Fatalf("after subscribe: count = %d, want 1", n)
	}

	unsub()
	if n := hub.subscriberCount("user"); n != 0 {
		t.Fatalf("after unsubscribe: count = %d, want 0", n)
	}

	unsub() // idempotent: must not panic or alter state
	if n := hub.subscriberCount("user"); n != 0 {
		t.Fatalf("after double unsubscribe: count = %d, want 0", n)
	}
}
