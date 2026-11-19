package rep_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/diego-logging-client/testhelpers"
	fakeecrhelper "code.cloudfoundry.org/ecrhelper/fakes"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/containerstore"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/rep"
	"github.com/gogo/protobuf/proto"
	"github.com/onsi/gomega"
)

func TestServiceAccountRuntimeContractRequiresPrefix(t *testing.T) {
	for _, kind := range []string{"LRP", "task"} {
		t.Run(kind, func(t *testing.T) {
			gomega.RegisterTestingT(t)
			properties := &models.CertificateProperties{ServiceAccount: &models.ServiceAccount{Name: "payments-worker"}}
			var err error
			if kind == "LRP" {
				desired := model_helpers.NewValidDesiredLRP("app-one")
				desired.CertificateProperties = properties
				err = desired.Validate()
			} else {
				task := model_helpers.NewValidTask("runtime-task")
				task.CertificateProperties = properties
				err = task.TaskDefinition.Validate()
			}
			if err == nil || !strings.Contains(err.Error(), "certificate_properties.service_account.prefix") {
				t.Fatalf("expected runtime contract to reject missing prefix, got: %v", err)
			}
		})
	}
}

func TestServiceAccountWireToInstanceCredentials(t *testing.T) {
	gomega.RegisterTestingT(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test-ca"}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	ca, err = x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	logger := lagertest.NewTestLogger("account-wire")
	manager := containerstore.NewCredManager(logger, &testhelpers.FakeIngressClient{}, time.Minute, rand.Reader, clock.NewClock(), ca, key, nil, containerstore.WithServiceAccountIdentity(true))
	helper := rep.RunRequestConversionHelper{ECRHelper: &fakeecrhelper.FakeECRHelper{}}
	stack := rep.StackPathMap{"stack": "stack:/rootfs"}
	account := &models.CertificateProperties{OrganizationalUnit: []string{"app:app-guid", "space:space-guid", "organization:org-guid"}, ServiceAccount: &models.ServiceAccount{Name: "payments-worker"}}
	if err := json.Unmarshal([]byte(`{"service_account":{"name":"payments-worker","prefix":"payments-team"},"organizational_unit":["app:app-guid","space:space-guid","organization:org-guid"]}`), account); err != nil {
		t.Fatal(err)
	}
	requests := []executor.RunRequest{}
	for _, guid := range []string{"app-one", "app-two"} {
		desired := model_helpers.NewValidDesiredLRP(guid)
		desired.RootFs = "preloaded:stack"
		desired.CertificateProperties = account
		wire, err := proto.Marshal(desired)
		if err != nil {
			t.Fatal(err)
		}
		decoded := &models.DesiredLRP{}
		if err := proto.Unmarshal(wire, decoded); err != nil {
			t.Fatal(err)
		}
		// Exercise the same split run-info assembly used by stored BBS LRPs.
		runInfo := decoded.DesiredLRPRunInfo(time.Now())
		decoded.CertificateProperties = nil
		decoded.AddRunInfo(runInfo)
		if err := decoded.Validate(); err != nil {
			t.Fatal(err)
		}
		actual := model_helpers.NewValidActualLRP(guid, 0)
		request, err := helper.NewRunRequestFromDesiredLRP(guid, decoded, &actual.ActualLRPKey, &actual.ActualLRPInstanceKey, stack, rep.LayeringModeSingleLayer)
		if err != nil {
			t.Fatal(err)
		}
		requests = append(requests, request)
	}
	task := model_helpers.NewValidTask("runtime-task")
	task.RootFs = "preloaded:stack"
	task.CertificateProperties = account
	wire, err := proto.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	decodedTask := &models.Task{}
	if err := proto.Unmarshal(wire, decodedTask); err != nil {
		t.Fatal(err)
	}
	if err := decodedTask.TaskDefinition.Validate(); err != nil {
		t.Fatal(err)
	}
	request, err := helper.NewRunRequestFromTask(decodedTask, stack, rep.LayeringModeSingleLayer)
	if err != nil {
		t.Fatal(err)
	}
	requests = append(requests, request)
	keys := map[string]bool{}
	for _, request := range requests {
		container := executor.Container{Guid: request.Guid, RunInfo: request.RunInfo, InternalIP: "10.0.0.1"}
		credentials, err := manager.GenerateInitialCredentials(logger, container)
		if err != nil {
			t.Fatal(err)
		}
		for _, credential := range []containerstore.Credential{credentials.InstanceIdentityCredential, credentials.C2CCredential} {
			block, _ := pem.Decode([]byte(credential.Cert))
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, name := range cert.DNSNames {
				if name == "payments-worker.payments-team.svc.identity" {
					count++
				}
			}
			if count != 1 || cert.Subject.CommonName != request.Guid {
				t.Fatalf("lost or ambiguous wire identity: %v", cert.DNSNames)
			}
			if len(cert.Subject.OrganizationalUnit) != 3 {
				t.Fatal("lost caller organizational units")
			}
			if keys[credential.Key] {
				t.Fatal("different containers reused a credential key")
			}
			keys[credential.Key] = true
		}
	}
}
