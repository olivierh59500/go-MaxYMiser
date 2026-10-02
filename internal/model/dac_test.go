package model

import "testing"

func TestNativeDACTableThresholds(t *testing.T) {
	for _, test := range []struct{ sample, level byte }{{0, 13}, {7, 13}, {8, 14}, {82, 14}, {83, 15}, {127, 15}, {128, 0}, {129, 1}, {130, 2}, {131, 3}, {170, 10}, {171, 11}, {216, 12}, {217, 13}, {255, 13}} {
		if got := DACLevel(test.sample); got != test.level {
			t.Fatalf("sample %d level %d, want %d", test.sample, got, test.level)
		}
	}
}
