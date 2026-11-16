package server

import (
	"testing"
	"time"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	now := time.Now()
	token, err := SignAccessToken(secret, 42, 3, now)
	if err != nil {
		t.Fatal(err)
	}

	claims, err := ParseAccessToken(secret, token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != 42 || claims.TokenVersion != 3 {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	if _, err := ParseAccessToken([]byte("wrong-secret-wrong-secret-wrong"), token); err == nil {
		t.Fatal("expected invalid signature")
	}
}

func TestAccessTokenExpired(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	token, err := SignAccessToken(secret, 42, 3, time.Now().Add(-accessTokenTTL-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAccessToken(secret, token); err == nil {
		t.Fatal("expected expired token")
	}
}
