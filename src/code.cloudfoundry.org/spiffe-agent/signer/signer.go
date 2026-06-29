// Package signer obtains JWT-SVIDs from the UAA /jwt-svid/sign endpoint,
// proving possession of the workload's instance key over the SPIFFE ID,
// audience, and timestamp. The wire contract is authoritative against Plan A
// (UAA Java): canonical message and key names must byte-match.
package signer

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"code.cloudfoundry.org/spiffe-agent/cfattestor"
)

// Signer exchanges a verified attestation for a JWT-SVID from UAA.
type Signer struct {
	httpClient   *http.Client
	url          string
	clientID     string
	clientSecret string
	trustDomain  string
}

// New returns a Signer that POSTs to url/jwt-svid/sign with HTTP Basic auth.
func New(c *http.Client, url, clientID, clientSecret, trustDomain string) *Signer {
	return &Signer{
		httpClient:   c,
		url:          url,
		clientID:     clientID,
		clientSecret: clientSecret,
		trustDomain:  trustDomain,
	}
}

type signRequest struct {
	InstanceCertificate string `json:"instance_certificate"`
	ProcessType         string `json:"process_type"`
	Audience            string `json:"audience"`
	Timestamp           string `json:"timestamp"`
	PopSignature        string `json:"pop_signature"`
}

type signResponse struct {
	Svid      string `json:"svid"`
	SpiffeID  string `json:"spiffe_id"`
	ExpiresAt string `json:"expires_at"`
}

// Sign proves possession of att.InstanceKey over the SPIFFE ID, audience, and a
// single captured timestamp, then exchanges it for a JWT-SVID. The timestamp is
// signed and sent verbatim so UAA can reverify the proof-of-possession.
func (s *Signer) Sign(ctx context.Context, att cfattestor.Attestation, audience string) (svid, spiffeID string, err error) {
	spiffeID = cfattestor.BuildSpiffeID(s.trustDomain, att.Selectors)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	message := spiffeID + "\n" + audience + "\n" + timestamp
	pop, err := signPoP(att.InstanceKey, message)
	if err != nil {
		return "", "", err
	}

	reqBody := signRequest{
		InstanceCertificate: att.CertPEM,
		ProcessType:         att.Selectors.ProcessType,
		Audience:            audience,
		Timestamp:           timestamp,
		PopSignature:        pop,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url+"/jwt-svid/sign", bytes.NewReader(payload))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(s.clientID, s.clientSecret)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("jwt-svid sign: status %d: %s", resp.StatusCode, body)
	}

	var out signResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", err
	}
	return out.Svid, out.SpiffeID, nil
}

// signPoP produces a STANDARD base64 SHA256withRSA PKCS#1 v1.5 signature over
// message. The key must be RSA.
func signPoP(key crypto.Signer, message string) (string, error) {
	if _, ok := key.Public().(*rsa.PublicKey); !ok {
		return "", fmt.Errorf("pop: instance key is not RSA")
	}
	sum := sha256.Sum256([]byte(message))
	sig, err := key.Sign(rand.Reader, sum[:], crypto.SHA256)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}
