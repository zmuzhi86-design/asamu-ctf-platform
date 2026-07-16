package user

import "testing"

func TestPrivacyDefaultsToVisibleAndHonorsFalse(t *testing.T) {
	if privacyOff(map[string]any{}, "showSkills") {
		t.Fatal("missing privacy preference must preserve compatibility")
	}
	if !privacyOff(map[string]any{"showSkills": false}, "showSkills") {
		t.Fatal("explicit false was ignored")
	}
	if privacyOff(map[string]any{"showSkills": true}, "showSkills") {
		t.Fatal("explicit true was hidden")
	}
}
