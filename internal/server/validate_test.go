package server

import "testing"

func TestProjectNamePattern(t *testing.T) {
	valid := []string{"a", "abc-123", "a_b", "0service"}
	invalid := []string{"", "-abc", "Abc", "../etc", "a b", "toolongprojectname123456789012345678901234567890123456789012345678901234567890"}

	for _, name := range valid {
		if !projectNamePattern.MatchString(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}
	for _, name := range invalid {
		if projectNamePattern.MatchString(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}
