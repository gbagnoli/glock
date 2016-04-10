package glock

import "testing"

func testClient(t *testing.T, cfun newClientFunc) {
	c1 := cfun(t)
	c2 := cfun(t)

	if c1.ID() == c2.ID() {
		t.Errorf("Both client have the same id: %s == %s", c1.ID(), c2.ID())
	}

	c1.SetID("myclient")
	id := c1.ID()
	if id != "myclient" {
		t.Errorf("SetID did not set ID correctly: 'myclient' expected, got %s", id)
	}

	// close should be idempotent, as well as Reconnect
	c1.Close()
	c1.Close()
	err := c1.Reconnect()
	if err != nil {
		t.Fatalf("Reconnect error: %s", err)
	}
	err = c1.Reconnect()
	if err != nil {
		t.Fatalf("Reconnect error: %s", err)
	}

	c3 := c1.Clone()
	if &c1 == &c3 {
		t.Fatalf("Clone() should clone, not returning the same object! %p == %p", &c1, &c3)
	}
	if c1.ID() != c3.ID() {
		t.Errorf("Clone should have copied client ids '%s' != '%s'", c1.ID(), c3.ID())
	}
}
