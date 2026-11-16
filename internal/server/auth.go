package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const accessTokenTTL = 30 * time.Minute

type AccessClaims struct {
	UserID       int64
	TokenVersion int64
	ExpiresAt    int64
}

func SignAccessToken(secret []byte, userID int64, tokenVersion int64, now time.Time) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := fmt.Sprintf(`{"sub":%d,"tv":%d,"exp":%d}`, userID, tokenVersion, now.Add(accessTokenTTL).Unix())
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(sign(secret, header+"."+encodedPayload))
	return header + "." + encodedPayload + "." + signature, nil
}

func ParseAccessToken(secret []byte, token string) (AccessClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return AccessClaims{}, errors.New("invalid token")
	}

	expected := base64.RawURLEncoding.EncodeToString(sign(secret, parts[0]+"."+parts[1]))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return AccessClaims{}, errors.New("invalid signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AccessClaims{}, errors.New("invalid token")
	}
	var claims struct {
		Sub int64 `json:"sub"`
		TV  int64 `json:"tv"`
		Exp int64 `json:"exp"`
	}
	if err := jsonUnmarshal(payload, &claims); err != nil {
		return AccessClaims{}, errors.New("invalid token")
	}
	if time.Now().After(time.Unix(claims.Exp, 0)) {
		return AccessClaims{}, errors.New("expired token")
	}
	return AccessClaims{UserID: claims.Sub, TokenVersion: claims.TV, ExpiresAt: claims.Exp}, nil
}

func sign(secret []byte, value string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func bearerToken(r *http.Request) string {
	value := r.Header.Get("Authorization")
	return strings.TrimPrefix(value, "Bearer ")
}

func parseID(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}
