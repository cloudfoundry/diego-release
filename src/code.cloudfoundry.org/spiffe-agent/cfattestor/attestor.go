package cfattestor

import (
	"bufio"
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Attestation is the full result of attesting a workload pid.
type Attestation struct {
	Selectors    Selectors
	CertPEM      string
	InstanceCert *x509.Certificate
	InstanceKey  crypto.Signer
}

// ProcessTypeResolver resolves the CF process type for a container handle.
type ProcessTypeResolver interface {
	ProcessType(ctx context.Context, handle string) (string, error)
}

// Attestor turns a pid into an Attestation using the container's cgroup,
// instance credentials, and process tree.
type Attestor struct {
	procRoot string
	resolver ProcessTypeResolver
}

// New returns an Attestor that reads from /proc.
func New(resolver ProcessTypeResolver) *Attestor {
	return &Attestor{procRoot: "/proc", resolver: resolver}
}

// Attest resolves the workload identity for pid. SSH sessions are rejected.
func (a *Attestor) Attest(ctx context.Context, pid int) (Attestation, error) {
	handle, err := containerHandle(a.procRoot, pid)
	if err != nil {
		return Attestation{}, err
	}

	ssh, err := hasSSHAncestor(a.procRoot, pid)
	if err != nil {
		return Attestation{}, err
	}
	if ssh {
		return Attestation{}, fmt.Errorf("ssh sessions are not attestable")
	}

	certPEM, cert, key, err := readInstanceCredentials(a.procRoot, pid)
	if err != nil {
		return Attestation{}, err
	}

	selectors, err := parseSelectors(cert)
	if err != nil {
		return Attestation{}, err
	}

	procType, err := a.resolver.ProcessType(ctx, handle)
	if err != nil {
		return Attestation{}, err
	}
	selectors.ProcessType = procType

	return Attestation{Selectors: selectors, CertPEM: certPEM, InstanceCert: cert, InstanceKey: key}, nil
}

// hasSSHAncestor walks the process tree from pid upward (max 20 hops, stopping
// at pid 1) and returns true if any ancestor is diego-sshd.
func hasSSHAncestor(procRoot string, pid int) (bool, error) {
	for hops := 0; hops < 20 && pid > 1; hops++ {
		name, ppid, err := readStatus(procRoot, pid)
		if err != nil {
			return false, err
		}
		if name == "diego-sshd" {
			return true, nil
		}
		pid = ppid
	}
	return false, nil
}

// readStatus parses Name: and PPid: from <procRoot>/<pid>/status.
func readStatus(procRoot string, pid int) (name string, ppid int, err error) {
	f, err := os.Open(filepath.Join(procRoot, strconv.Itoa(pid), "status"))
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "Name:"):
			name = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
		case strings.HasPrefix(line, "PPid:"):
			ppid, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "PPid:")))
		}
	}
	return name, ppid, sc.Err()
}

// BuildSpiffeID builds the canonical CF SPIFFE ID for s.
func BuildSpiffeID(trustDomain string, s Selectors) string {
	return fmt.Sprintf("spiffe://%s/cf/org/%s/space/%s/app/%s/process/%s",
		trustDomain, s.OrgID, s.SpaceID, s.AppID, s.ProcessType)
}
