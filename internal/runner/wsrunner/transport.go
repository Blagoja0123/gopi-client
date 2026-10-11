package wsrunner

import (
	"crypto/tls"
	"net/http"
)

func insecureTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // user opt-in
	return t
}
