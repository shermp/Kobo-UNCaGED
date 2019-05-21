package main

import "testing"

func TestKoboDeviceValues(t *testing.T) {
	for _, device := range []koboDevice{
		touchAB, touchC, mini, glo, gloHD, touch2, aura, auraHD, auraH2O,
		auraH2Oed2r1, auraH2Oed2r2, auraOne, auraOneLE, auraEd2r1, auraEd2r2,
		claraHD, forma,
	} {
		if len(string(device)) != 36 {
			t.Errorf("expected device id to be 36 long for %#v", device)
		}
		if device.Model() == "" || device.Model() == "Unknown Kobo" {
			t.Errorf("expected non-blank model for %#v", device)
		}
	}
}
