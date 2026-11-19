package executor_test

import (
	"testing"

	"code.cloudfoundry.org/executor"
)

func TestContainerCopyIsolatesServiceAccountIdentity(t *testing.T) {
	original := executor.Container{RunInfo: executor.RunInfo{CertificateProperties: executor.CertificateProperties{ServiceAccount: &executor.ServiceAccount{Name: "payments-worker", Prefix: "payments-team"}}}}
	copy := original.Copy()
	copy.CertificateProperties.ServiceAccount.Name = "reporting-reader"
	copy.CertificateProperties.ServiceAccount.Prefix = "reporting-team"
	if original.CertificateProperties.ServiceAccount.Name != "payments-worker" || original.CertificateProperties.ServiceAccount.Prefix != "payments-team" {
		t.Fatal("container copy exposes mutable launch account identity")
	}
	if executor.NewContainerFromResource("unbound", &executor.Resource{}, nil).Copy().CertificateProperties.ServiceAccount != nil {
		t.Fatal("copy introduced an account to an unbound container")
	}
}
