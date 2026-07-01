package envoyconfig

import (
	"bytes"
	"encoding/pem"
	"strings"
	"testing"

	"code.cloudfoundry.org/executor"
)

func TestAssignProxyPorts_SinglePort(t *testing.T) {
	ports := []executor.PortMapping{{ContainerPort: 9090}}
	out, err := AssignProxyPorts(ports)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(out))
	}
	if out[0].ContainerPort != 9090 {
		t.Errorf("expected ContainerPort 9090, got %d", out[0].ContainerPort)
	}
	if out[0].ContainerTLSProxyPort != StartProxyPort {
		t.Errorf("expected ContainerTLSProxyPort %d, got %d", StartProxyPort, out[0].ContainerTLSProxyPort)
	}
	if out[0].HostPort != 0 || out[0].HostTLSProxyPort != 0 {
		t.Error("HostPort and HostTLSProxyPort should be 0")
	}
}

func TestAssignProxyPorts_Port8080GetsC2CMapping(t *testing.T) {
	ports := []executor.PortMapping{{ContainerPort: 8080}, {ContainerPort: 9090}}
	out, err := AssignProxyPorts(ports)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 mappings (8080->61001, 8080->61443, 9090->61002), got %d", len(out))
	}
	if out[0].ContainerPort != 8080 || out[0].ContainerTLSProxyPort != 61001 {
		t.Errorf("mapping[0] = %+v", out[0])
	}
	if out[1].ContainerPort != 8080 || out[1].ContainerTLSProxyPort != C2CTLSPort {
		t.Errorf("mapping[1] = %+v", out[1])
	}
	if out[2].ContainerPort != 9090 || out[2].ContainerTLSProxyPort != 61002 {
		t.Errorf("mapping[2] = %+v", out[2])
	}
}

func TestAssignProxyPorts_SkipsCollidingPorts(t *testing.T) {
	ports := []executor.PortMapping{{ContainerPort: 61001}, {ContainerPort: 9090}}
	out, err := AssignProxyPorts(ports)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(out))
	}
	if out[0].ContainerTLSProxyPort == 61001 {
		t.Error("proxy port should not collide with app port 61001")
	}
}

func TestAssignProxyPorts_DeduplicatesInput(t *testing.T) {
	ports := []executor.PortMapping{{ContainerPort: 8080}, {ContainerPort: 8080}}
	out, err := AssignProxyPorts(ports)
	if err != nil {
		t.Fatal(err)
	}
	// 8080 appears once -> 2 mappings (61001 + 61443)
	if len(out) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(out))
	}
}

func TestGetAvailablePort(t *testing.T) {
	ports := []executor.PortMapping{
		{ContainerPort: 8080, ContainerTLSProxyPort: 61001},
		{ContainerPort: 8080, ContainerTLSProxyPort: 61443},
	}
	p, err := GetAvailablePort(ports)
	if err != nil {
		t.Fatal(err)
	}
	if p < StartProxyPort || p >= EndProxyPort {
		t.Errorf("port %d out of range", p)
	}
	if p == 61001 || p == 61443 {
		t.Errorf("port should not be in allocated list")
	}
}

func TestGenerateBootstrap(t *testing.T) {
	container := executor.Container{
		Guid:       "meow-guid",
		InternalIP: "10.10.10.10",
		RunInfo: executor.RunInfo{
			Ports: []executor.PortMapping{
				{ContainerPort: 8080, ContainerTLSProxyPort: 61001},
				{ContainerPort: 8080, ContainerTLSProxyPort: 61443},
			},
		},
	}
	yaml, err := GenerateBootstrap(container, 61000, false, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(yaml) == 0 {
		t.Error("expected non-empty YAML")
	}
	if !bytes.Contains(yaml, []byte("10.10.10.10")) || !bytes.Contains(yaml, []byte("meow-guid")) || !bytes.Contains(yaml, []byte("envoy_config")) {
		n := 200
		if len(yaml) < n {
			n = len(yaml)
		}
		t.Errorf("YAML missing expected content: %s", string(yaml)[:n])
	}
}

func TestGenerateSDSCertAndKey(t *testing.T) {
	cred := Credential{Cert: "potato-cert", Key: "potato-key"}
	yaml, err := GenerateSDSCertAndKey("banana", cred)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(yaml, []byte("potato-cert")) || !bytes.Contains(yaml, []byte("potato-key")) {
		t.Error("YAML missing cert/key")
	}
}

func TestGenerateSDSCAResource(t *testing.T) {
	container := executor.Container{Guid: "meow-guid"}
	idCred := Credential{}
	trusted := []string{}
	yaml, err := GenerateSDSCAResource(container, idCred, trusted, []string{"some-guid.cf.internal", "gorouter.service.cf.internal"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(yaml, []byte("id-validation-context")) {
		t.Error("YAML missing validation context name")
	}
	if !bytes.Contains(yaml, []byte("some-guid.cf.internal")) {
		t.Error("YAML missing first DNS SAN")
	}
	if !bytes.Contains(yaml, []byte("gorouter.service.cf.internal")) {
		t.Error("YAML missing second DNS SAN")
	}
}

func TestGenerateSDSCAResource_TrustedCACertsTooLarge(t *testing.T) {
	container := executor.Container{Guid: "meow-guid"}
	idCred := Credential{}
	bigBody := bytes.Repeat([]byte("A"), maxTrustedCABytes+1)
	block := &pem.Block{Type: "CERTIFICATE", Bytes: bigBody}
	bigPEM := string(pem.EncodeToMemory(block))
	trusted := []string{bigPEM}
	_, err := GenerateSDSCAResource(container, idCred, trusted, nil)
	if err == nil {
		t.Fatal("expected error when trusted CA exceeds max size")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("expected exceeds maximum size error, got: %v", err)
	}
}

func TestPemConcatenate_MultipleBlocksInOneCert(t *testing.T) {
	// A single cert string containing 2 PEM blocks (e.g. chain cert).
	// Before the fix, `:=` in the decode loop shadowed `rest`, causing an infinite loop.
	block1 := &pem.Block{Type: "CERTIFICATE", Bytes: []byte("potato")}
	block2 := &pem.Block{Type: "CERTIFICATE", Bytes: []byte("banana")}
	certPEM := string(pem.EncodeToMemory(block1)) + string(pem.EncodeToMemory(block2))

	result, err := pemConcatenate([]string{certPEM})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rest := []byte(result)
	count := 0
	for {
		var b *pem.Block
		b, rest = pem.Decode(rest)
		if b == nil {
			break
		}
		count++
	}
	if count != 2 {
		t.Errorf("expected 2 PEM blocks in output, got %d", count)
	}
}

func TestPemConcatenate_MultipleCertStrings(t *testing.T) {
	block1 := &pem.Block{Type: "CERTIFICATE", Bytes: []byte("meow")}
	block2 := &pem.Block{Type: "CERTIFICATE", Bytes: []byte("potato")}
	cert1 := string(pem.EncodeToMemory(block1))
	cert2 := string(pem.EncodeToMemory(block2))

	result, err := pemConcatenate([]string{cert1, cert2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rest := []byte(result)
	count := 0
	for {
		var b *pem.Block
		b, rest = pem.Decode(rest)
		if b == nil {
			break
		}
		count++
	}
	if count != 2 {
		t.Errorf("expected 2 PEM blocks in output, got %d", count)
	}
}

func TestPemConcatenate_SkipsEmptyStrings(t *testing.T) {
	block := &pem.Block{Type: "CERTIFICATE", Bytes: []byte("meow")}
	cert := string(pem.EncodeToMemory(block))

	result, err := pemConcatenate([]string{"", cert, ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains([]byte(result), []byte("CERTIFICATE")) {
		t.Error("expected PEM output")
	}
}
