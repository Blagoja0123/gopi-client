package webhook

import (
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // many webhook providers still use HMAC-SHA1
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"strconv"
	"strings"
	"time"
)

func hasher(algo string) (func() hash.Hash, error) {
	switch strings.ToLower(algo) {
	case "sha1":
		return sha1.New, nil
	case "sha256", "":
		return sha256.New, nil
	case "sha512":
		return sha512.New, nil
	}
	return nil, fmt.Errorf("unsupported algorithm %q (use sha1, sha256 or sha512)", algo)
}

// Sign returns the hex HMAC of payload.
func Sign(algo, secret string, payload []byte) (string, error) {
	h, err := hasher(algo)
	if err != nil {
		return "", err
	}
	m := hmac.New(h, []byte(secret))
	m.Write(payload)
	return hex.EncodeToString(m.Sum(nil)), nil
}

// GitHubSignature returns the X-Hub-Signature-256 header value.
func GitHubSignature(secret string, payload []byte) string {
	s, _ := Sign("sha256", secret, payload)
	return "sha256=" + s
}

func VerifyGitHub(secret string, payload []byte, header string) bool {
	return hmac.Equal([]byte(GitHubSignature(secret, payload)), []byte(header))
}

// StripeSignature returns a Stripe-Signature header value: t=<ts>,v1=<hmac>
// where the HMAC is over "<ts>.<payload>".
func StripeSignature(secret string, payload []byte, at time.Time) string {
	ts := strconv.FormatInt(at.Unix(), 10)
	s, _ := Sign("sha256", secret, append([]byte(ts+"."), payload...))
	return "t=" + ts + ",v1=" + s
}

func VerifyStripe(secret string, payload []byte, header string, tolerance time.Duration, now time.Time) error {
	var ts string
	var sigs []string
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			ts = v
		case "v1":
			sigs = append(sigs, v)
		}
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || len(sigs) == 0 {
		return errors.New("malformed Stripe-Signature header")
	}
	if d := now.Sub(time.Unix(sec, 0)); d > tolerance || d < -tolerance {
		return fmt.Errorf("timestamp outside tolerance (%s)", d.Round(time.Second))
	}
	want, _ := Sign("sha256", secret, append([]byte(ts+"."), payload...))
	for _, s := range sigs {
		if hmac.Equal([]byte(s), []byte(want)) {
			return nil
		}
	}
	return errors.New("no matching v1 signature")
}
