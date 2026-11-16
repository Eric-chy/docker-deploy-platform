package server

import "testing"

func TestValidPasswordFormat(t *testing.T) {
	validPasswords := []string{"pass123456", "PASS123456", "pass!23456", "12345678!"}
	for _, password := range validPasswords {
		if !validPasswordFormat(password) {
			t.Errorf("expected %q to be valid", password)
		}
	}

	invalidPasswords := []string{"pass", "password", "12345678", "!!!!!!!!", "中文密码123", "PASS123456789012345"}
	for _, password := range invalidPasswords {
		if validPasswordFormat(password) {
			t.Errorf("expected %q to be invalid", password)
		}
	}
}
