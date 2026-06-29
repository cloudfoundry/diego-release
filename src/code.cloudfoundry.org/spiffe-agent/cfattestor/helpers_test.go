package cfattestor

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	. "github.com/onsi/gomega"
)

// writeCgroup writes a fake <procRoot>/<pid>/cgroup file.
func writeCgroup(procRoot string, pid int, content string) {
	dir := filepath.Join(procRoot, strconv.Itoa(pid))
	ExpectWithOffset(1, os.MkdirAll(dir, 0755)).To(Succeed())
	ExpectWithOffset(1, os.WriteFile(filepath.Join(dir, "cgroup"), []byte(content), 0644)).To(Succeed())
}

// writeStatus writes a fake <procRoot>/<pid>/status with Name: and PPid: lines.
func writeStatus(procRoot string, pid int, name string, ppid int) {
	dir := filepath.Join(procRoot, strconv.Itoa(pid))
	ExpectWithOffset(1, os.MkdirAll(dir, 0755)).To(Succeed())
	content := fmt.Sprintf("Name:\t%s\nPPid:\t%d\n", name, ppid)
	ExpectWithOffset(1, os.WriteFile(filepath.Join(dir, "status"), []byte(content), 0644)).To(Succeed())
}

// setupContainer wires up a fake pid: cgroup with the handle, a non-ssh status
// (Name=web, PPid=1), and instance credentials carrying s. Certs are minted
// inline via mintInstanceCert because a white-box test cannot import testsupport.
func setupContainer(procRoot string, pid int, handle string, s Selectors) {
	writeCgroup(procRoot, pid, "0::/garden/"+handle+"/init\n")
	writeStatus(procRoot, pid, "web", 1)

	certPEM, keyPEM, _ := mintInstanceCert(handle, s)
	credDir := filepath.Join(procRoot, strconv.Itoa(pid), "root", "etc", "cf-instance-credentials")
	ExpectWithOffset(1, os.MkdirAll(credDir, 0755)).To(Succeed())
	ExpectWithOffset(1, os.WriteFile(filepath.Join(credDir, "instance.crt"), []byte(certPEM), 0644)).To(Succeed())
	ExpectWithOffset(1, os.WriteFile(filepath.Join(credDir, "instance.key"), []byte(keyPEM), 0600)).To(Succeed())
}
