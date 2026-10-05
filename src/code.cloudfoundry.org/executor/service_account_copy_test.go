package executor_test

import (
	"testing"

	"code.cloudfoundry.org/executor"
)

func TestContainerCopyIsolatesServiceAccountIdentity(t *testing.T) {
	original := executor.Container{RunInfo: executor.RunInfo{CertificateProperties: executor.CertificateProperties{ServiceAccount: &executor.ServiceAccount{Name: "payments-worker"}}}}
	copy := original.Copy()
	copy.CertificateProperties.ServiceAccount.Name = "reporting-reader"
	if original.CertificateProperties.ServiceAccount.Name != "payments-worker" {
		t.Fatal("container copy exposes mutable launch account identity")
	}
	if executor.NewContainerFromResource("unbound", &executor.Resource{}, nil).Copy().CertificateProperties.ServiceAccount != nil {
		t.Fatal("copy introduced an account to an unbound container")
	}
}
