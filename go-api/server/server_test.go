package server

import "testing"

func TestRatesScaled(t *testing.T) {
	scaled := DefaultRates.Scaled(10)
	if scaled.AuthBurst != DefaultRates.AuthBurst*10 || scaled.ChatPerMinute != DefaultRates.ChatPerMinute*10 ||
		scaled.ExportBurst != DefaultRates.ExportBurst*10 {
		t.Errorf("limits were not multiplied: %+v", scaled)
	}
	for _, factor := range []int{0, 1, -3} {
		if DefaultRates.Scaled(factor) != DefaultRates {
			t.Errorf("a factor of %d must leave the limits unchanged", factor)
		}
	}
}
