package native

import (
	"encoding/binary"
	"testing"
)

func copySelectorFixture() []byte {
	b := make([]byte, 512)
	at := 16
	copy(b[at:], []byte{0x53, 0x40, 0xe5, 0x40, 0x41, 0xfa, 0, 0, 0xd1, 0xfc, 0, 0, 0, 0, 0xd1, 0xfc, 0, 0, 0, 0, 0xd1, 0xf0, 0, 0})
	binary.BigEndian.PutUint16(b[at+6:], uint16(256-at-6))
	binary.BigEndian.PutUint32(b[256:], 384-8-256)
	binary.BigEndian.PutUint32(b[260:], 320-8-256)
	return b
}

func TestCopySelectorUsesVerifiedTableOrderRatherThanPhysicalOrder(t *testing.T) {
	projects := []EmbeddedProject{{Title: "physical first"}, {Title: "physical second"}}
	got := reorderCopySelector(copySelectorFixture(), projects, []int{320, 384})
	if got[0].Title != "physical second" || got[1].Title != "physical first" {
		t.Fatal("native selector table order was not honored")
	}
	if projects[0].Title != "physical first" {
		t.Fatal("selector mutated the decoded source ordering")
	}
}

func TestCopySelectorRejectsUnresolvedOrDuplicatePointers(t *testing.T) {
	projects := []EmbeddedProject{{Title: "first"}, {Title: "second"}}
	for _, pointer := range []uint32{0xffffffff, 384 - 8 - 256} {
		b := copySelectorFixture()
		binary.BigEndian.PutUint32(b[260:], pointer)
		got := reorderCopySelector(b, projects, []int{320, 384})
		if got[0].Title != "first" {
			t.Fatal("invalid selector invented an executable song mapping")
		}
	}
	for n := 0; n < 40; n++ {
		reorderCopySelector(copySelectorFixture()[:n], projects, []int{320, 384})
	}
}
