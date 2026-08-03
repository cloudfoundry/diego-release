package vizzini_test

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/kr/pty"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"code.cloudfoundry.org/bbs/models"
	ssh_routes "code.cloudfoundry.org/diego-ssh/routes"

	. "code.cloudfoundry.org/vizzini/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const userPrivateRSAKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEhgIBAAKB/C/hstPGznfdyUGdbatKgbWJYRTb8S8A7ehto1SukBzCKrR+Dw5I
y/qSIzi82xkOGjckEECa2B9fiACBY+fQQPvInCnU5iMUkJNZcrugJhnv6S9y8k3U
t7HT9YVlIxDpjxyxdrkkkmoPCAu0zSqUQuv6QlKBi2A7wZcfwmupOue11vhaPQ+K
NULtJaiYNQoHsvO/hxe/wcKmHI4R0cWp/zibNqx5xz6eaao5qsrshr02mRxMumYC
QohfM93/wL+oVyzLMSeaKxZtAglfMecjNcUn9Sk22Jq1bbvu8cLR9Gdg35XeHl5G
if03/JQsXbUpLeQd8nXKUjYk8uNAHQIDAQABAoH8FOC0uOLW5C0wtAuQ5j92j1F3
o0DDyVr+YXps3V/ANsnzFQBiUDgtuPQ/p12xqxsbEzAGZiUeV4+wHYhNp6aGr0Kp
1ROfxWwSHi3CeU07T9PsOWRFgupdroxdYezXfWhZnolC2ze3H8euGmybiRVcmMhm
YtNZknx7zQlsHMWNKSasBI0oKks7JLLuIF4eapdwnlMcw7PxO8rUs/3K6psbsiN0
AA5J/5KlkEniT7NH+Frs0jmdj/3AkuMnYnj3izJsL72kHOFvNUMdcxZX7V1xoFcy
npD0CcgpYbw6dA83fglqQcl6VO9vWff4nZAdqPyqlQCDbNWvKPyDu7mBAn5r9tSu
s3optWwLhgC6WCr34Qg3NAzwTFZI3HXeP28urOlFTXLzvVJc/RRFVEHnmOczaULo
zopwywtfQpa0Z5NAYGxPn7DB1JahqjMNdW66h5UUcgCInd1rZRtsP8xikCJmKoqa
b7e8F0tVyXrwvJBDLKYY11IpcijgIHxERF8CfnGI7K/Ev4jGZ1FdOouGSQ+pbunO
UPSPU4pzNuT6Phtgyrkd1cArTzPvjLIo5e91z+HI/YBDkHsibTFkVXGL54LrHLnS
KwSKIUvjm8HT4GG85BQbjhb2RTGkJTb63LOXuBXYOoH9xdLU52u843zxtW0p77LP
JqD5mEpyJUZtAwJ+UDDoTFLW/D/a3rxLsh1m3PLyjT5GFf49YKUPj2KCjKK2KVmb
dls64ALCmbQ5t3Ik2FTo887lmV3XNoxZL+p2vyxfhszQF0h2EeI/RVHiSv4Fx0fe
CZtoKSrSMZc5kkQIqOYUSR2N1VFgDXo3rLQCW0LApFbamhpHLiIy6un1An4unkiB
i8oRwVXfJObLL6KEWc//FQZMxSVKbjCWKOKjn0Teag/AzofBDZW5+e0gPEHVtg/R
QOzsgqBPbaFf9FBlg2DSNCgRvx4Y6SalmfhCaatFTmMzrn+O+JWHU86Xt66Q2a58
fdVi0qULqg3G2gDjCBsyUrjL1HDh8Ki5mD0Cfj/Rhdjn5THUmPkujPY0PZUzEgEs
PrdeYY5DBlgxM2zFdHX466qYy7rPT/H2YXMqqoZMQnCXNa8t/kcPa2F1C9j3HI2k
Jm/15BLfU/Ty+MHchPV6bR6fQ6SnePDKQNOBSxtMQT8oFNNM/os+WYpsF5dG8whH
wWA9OrJdbrDo9w==
-----END RSA PRIVATE KEY-----`

const userAuthorizedKey = ` ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAA/C/hstPGznfdyUGdbatKgbWJYRTb8S8A7ehto1SukBzCKrR+Dw5Iy/qSIzi82xkOGjckEECa2B9fiACBY+fQQPvInCnU5iMUkJNZcrugJhnv6S9y8k3Ut7HT9YVlIxDpjxyxdrkkkmoPCAu0zSqUQuv6QlKBi2A7wZcfwmupOue11vhaPQ+KNULtJaiYNQoHsvO/hxe/wcKmHI4R0cWp/zibNqx5xz6eaao5qsrshr02mRxMumYCQohfM93/wL+oVyzLMSeaKxZtAglfMecjNcUn9Sk22Jq1bbvu8cLR9Gdg35XeHl5Gif03/JQsXbUpLeQd8nXKUjYk8uNAHQ==`

const hostPrivateRSAKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA2/Qt7gMd20+O+sOKxSCk+7AvFxJAqFNfnrB9nlAU1986UR3R
euk5xqTvJokv3yCbzTPP07cQFHenalBrg2/sjCdq9MWKi8vfIBW3malquLr0fBRx
qboukme2rgNXH0HCALxLinjBLPK5ToRMB25FVixkYo8nEvUbfiYH5GIRawXdRo7L
XXIK/Htx40zUPpIu9juB5K+RFovI+MIv6U3QqxNYOUhfbPhaGVV0FdFLO+uQNdLy
/ov7tI1mJVKTvpPR+Eea6v2rL75KdOfJ5prGVsAPrDA6gcvYpuJqaSi3OJe5sEhq
yQs9qQoA60tGQfX0bmRdEhWDiueCXW5/WoU/2wIDAQABAoIBACReF0oHUeR1Hxrv
Qf6eCyliVCboabBrOKAwZlTKwOeAjU/kMkK0VU028CPbAwNNjPU839wNpKb9sbyu
V1iAJQh3bAPUtbevmdDgRl8t1+t7Xfk2GCUMF681Xsse2kTcxosAlyzqEmawK1uE
HF4OKYC6Dk8NhFRqGoWdHCjy3hZnrzZsaNUUlMMCRTkyENCXR67hLT5TQfqEpbHP
5BVSJe3nmvAUPN7Q3Nk98aRAzC1NLv7XGKCvLNFF7L4Kvq3BfoXVOjekvyF/Ssbn
ciRpPRgxCJP3vlkWGFVbtnnZQpQDSc+LIbAxZ4BI5MMcsgX6YyyBbUzRbtub7JVQ
rW18GWECgYEA+95Ye4lsc5Tp+WK9JvxvGUAjgrtO4t8/v6129NnYD1miZCjwb8Sn
Ewk97cNfkWmTnCtsL5V18YXkpdgj3lF9cfnAFdJc3IX9xzpbrykEuJV1kxCCU61P
9LsehAOpNTDAwfM+wX92W6iNnYd+jBAiEPExP2qkKetnHfNVb8a5FB8CgYEA34/R
Bo4/Yb0E2Xi8C1FBOUlmDPhqN8QKKjdpVNSIrD5Q5eV1xpKKpZuB9TaCWQEWvcwT
nbXTGTJ4CUGSMzGMZg7iPREXt41YRdiv/VrBA/zevSC6OfhiJx+Mz32aDWNFfUtV
CwQlSSfiC5lw8PD9uY6q/lJUEBPcDXIBj2JuvMUCgYEAt4mONu+sjQlN+sIeDmPT
XbYkamauFJsUrEvurHx2erEZqh0/IGNQUInii/lcEe26eAoYexBR8x9bwBKiCKaf
YEfb1ssFillF1kFLgHfGje+zzugv4GQiKLeWhCLa0fzl6i+kYoLMr/xCvjF3YP98
o5XvCkRevoFhEi047AwG4IcCgYBz1OwUXXdxiKIOm4OyyXLl36XEaqF+K1Co9vTY
QxZdSBxaQT14mUzE6YG4L3nx66KAzFANkrvBfmi7QwIhDDcWWffWdBi5vb5S0ia9
OlxvWIF/tIlIp+0TIEGw7/71mM3UUUfK4WcANG3mXKYr8HFFxynJg5aSjfeh78Pn
KrT9kQKBgQCy9UfV4Kku8Zk6FeqWZvZP+wYarG9BMvc7C4mT+6bNMCeCmMydueZs
u6FLDjvicUuG1MZywSCoOpI6MZkcZiFXwdgIdRFdhDDcWdewItsJXBmHMjzr+t8P
hqA2YFwsUWCcgAxICpYQyTFVYBnHUVPYAHzctmWRbQuXhMJgWIRNhw==
-----END RSA PRIVATE KEY-----`

const hostFingerprint = `9e:33:35:e0:fe:67:e5:c4:7e:90:53:72:c2:3f:a1:9c`

type sshTarget struct {
	User string
	Host string
	Port string
}

var _ = Describe("SSH Tests", func() {
	var (
		password      string
		target        sshTarget
		lrp           *models.DesiredLRP
		user          string
		rootfs        string
		startTimeout  time.Duration
		gracePath     string
		sshdArgs      []string
		sshClientArgs []string
	)

	secureCommand := func(cmd string, args ...string) *exec.Cmd {
		sshArgs := []string{}
		sshArgs = append(sshArgs, sshClientArgs...)
		sshArgs = append(sshArgs, args...)

		return exec.Command(cmd, sshArgs...)
	}

	ssh := func(target sshTarget, args ...string) *exec.Cmd {
		sshArgs := []string{
			"-o", "User=" + target.User,
			"-p", target.Port,
			target.Host,
		}
		return secureCommand("ssh", append(sshArgs, args...)...)
	}

	sshInteractive := func(target sshTarget) *exec.Cmd {
		return ssh(target,
			"-t",
			"-t", // double tap to force pty allocation
		)
	}

	sshTunnelTo := func(target sshTarget, localport, remoteport int) *exec.Cmd {
		return ssh(target,
			"-N",
			"-L", fmt.Sprintf("%d:127.0.0.1:%d", localport, remoteport),
		)
	}

	scp := func(target sshTarget, args ...string) *exec.Cmd {
		scpArgs := []string{
			"-o", "User=" + target.User,
			"-P", target.Port,
			"-vvv",
		}
		return secureCommand("scp", append(scpArgs, args...)...)
	}

	sftp := func(target sshTarget, args ...string) *exec.Cmd {
		sftpArgs := []string{
			"-v",
			"-P", target.Port,
			"-o", "User=" + target.User,
			sshHost,
		}
		return secureCommand("sftp", append(sftpArgs, args...)...)
	}

	BeforeEach(func() {
		password = config.SSHPassword
		target = sshTarget{
			User: "diego:" + guid + "/0",
			Port: sshPort,
			Host: sshHost,
		}

		user = "vcap"
		startTimeout = timeout
		sshdArgs = append(sshdArgs,
			"-hostKey="+hostPrivateRSAKey,
			"-authorizedKey="+userAuthorizedKey,
			"-inheritDaemonEnv",
		)
		sshClientArgs = []string{
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
		}
	})

	JustBeforeEach(func() {
		sshRoutePayload, err := json.Marshal(ssh_routes.SSHRoute{
			ContainerPort:   2222,
			HostFingerprint: hostFingerprint,
			PrivateKey:      userPrivateRSAKey,
		})
		Expect(err).NotTo(HaveOccurred())

		sshRouteJSON := json.RawMessage(sshRoutePayload)
		routes := models.Routes{
			ssh_routes.DIEGO_SSH: &sshRouteJSON,
		}

		lrp = &models.DesiredLRP{
			ProcessGuid:          guid,
			PlacementTags:        PlacementTags(),
			Domain:               domain,
			Instances:            1,
			EnvironmentVariables: []*models.EnvironmentVariable{{Name: "CUMBERBUND", Value: "cummerbund"}},
			CachedDependencies: []*models.CachedDependency{
				&models.CachedDependency{
					Name:     "grace",
					From:     config.GraceTarballURL,
					To:       "/tmp/grace",
					CacheKey: "grace",
				},
				&models.CachedDependency{
					Name:     "lifecycle bundle",
					From:     config.FileServerAddress + "/v1/static/buildpack_app_lifecycle/buildpack_app_lifecycle.tgz",
					To:       "/tmp/lifecycle",
					CacheKey: "lifecycle",
				},
			},
			Action: models.WrapAction(models.Parallel(
				&models.RunAction{
					Path: "/tmp/lifecycle/diego-sshd",
					Args: append([]string{
						"-address=0.0.0.0:2222",
						"-logLevel=debug",
					}, sshdArgs...),
					User: user,
				},
				&models.RunAction{
					Path: gracePath,
					User: user,
					Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8888"}, {Name: "INSTANCE_INDEX", Value: "inconceivable!"}},
				},
			)),
			Monitor: models.WrapAction(models.Parallel(
				&models.RunAction{
					Path: "/tmp/lifecycle/healthcheck",
					Args: []string{"-port", "2222"},
					User: user,
				},
				&models.RunAction{
					Path: "/tmp/lifecycle/healthcheck",
					Args: []string{"-port", "8888"},
					User: user,
				},
			)),
			RootFs:     rootfs,
			MemoryMb:   128,
			DiskMb:     128,
			Ports:      []uint32{2222},
			Routes:     &routes,
			MetricTags: map[string]*models.MetricTagValue{"source_id": {Static: guid}},
		}

		Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
		Eventually(ActualGetter(logger, guid, 0), startTimeout).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
	})

	Context("in a fully-featured preloaded rootfs", func() {
		BeforeEach(func() {
			user = "vcap"
			rootfs = config.DefaultRootFS
			gracePath = "/tmp/grace/grace"
		})

		It("runs an ssh command", func() {
			cmd := ssh(target, "/usr/bin/env")

			session := runWithPassword(cmd, password)

			Eventually(session).Should(gexec.Exit(0))
			output := session.Out.Contents()
			Expect(output).To(ContainSubstring("USER=" + user))
			Expect(output).To(ContainSubstring("CUMBERBUND=cummerbund"))
		})

		It("runs an interactive ssh session", func() {
			cmd := sshInteractive(target)

			session := runInteractiveWithPassword(cmd, password, func(session *gexec.Session, input *os.File) {
				Eventually(session).Should(gbytes.Say(user + "@"))

				_, err := input.Write([]byte("export FOO=foo; echo ${FOO}bar\n"))
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gbytes.Say("foobar"))

				_, err = input.Write([]byte("exit\n"))
				Expect(err).NotTo(HaveOccurred())

				Eventually(session.Err).Should(gbytes.Say("Connection to " + target.Host + " closed."))
			})

			Eventually(session).Should(gexec.Exit(0))
		})

		It("forwards ports", func() {
			cmd := sshTunnelTo(target, 12345, 8888)

			session := runInteractiveWithPassword(cmd, password, func(session *gexec.Session, _ *os.File) {
				Eventually(session.Err).Should(gbytes.Say("Warning: Permanently added"))

				resp, err := http.Get("http://127.0.0.1:12345/index")
				Expect(err).NotTo(HaveOccurred())

				defer resp.Body.Close()

				payload, err := io.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(payload)).To(Equal("inconceivable!"))

				session.Interrupt()
			})

			Eventually(session).Should(gexec.Exit())
		})

		Describe("scp", func() {
			var (
				dir    string
				err    error
				inpath string
			)
			BeforeEach(func() {
				dir, err = os.MkdirTemp("", "vizzini-ssh")
				Expect(err).NotTo(HaveOccurred())

				inpath = path.Join(dir, "inbound")
				infile, err := os.Create(inpath)
				Expect(err).NotTo(HaveOccurred())

				_, err = infile.Write([]byte("hello from vizzini"))
				Expect(err).NotTo(HaveOccurred())

				err = infile.Close()
				Expect(err).NotTo(HaveOccurred())
			})
			AfterEach(func() {
				os.RemoveAll(dir)
			})
			It("copies files back and forth", func() {

				insession := runWithPassword(scp(target,
					inpath,
					target.Host+":in-container",
				), password)

				Eventually(insession).Should(gexec.Exit())

				outpath := path.Join(dir, "outbound")
				outsession := runWithPassword(scp(target,
					target.Host+":in-container",
					outpath,
				), password)

				Eventually(outsession).Should(gexec.Exit())

				contents, err := os.ReadFile(outpath)
				Expect(err).NotTo(HaveOccurred())
				Expect(contents).To(Equal([]byte("hello from vizzini")))
			})
		})

		Describe("sftp", func() {
			var (
				sourceDir, targetDir             string
				generatedFile, generatedFileName string
				err                              error
			)

			BeforeEach(func() {
				sourceDir, err = os.MkdirTemp("", "sftp-source")
				Expect(err).NotTo(HaveOccurred())

				fileContents := make([]byte, 1024)
				b, err := rand.Read(fileContents)
				Expect(err).NotTo(HaveOccurred())
				Expect(b).To(Equal(len(fileContents)))

				generatedFileName = "binary.dat"
				generatedFile = filepath.Join(sourceDir, generatedFileName)

				err = os.WriteFile(generatedFile, fileContents, 0644)
				Expect(err).NotTo(HaveOccurred())

				_, err = os.Stat(generatedFile)
				Expect(err).NotTo(HaveOccurred())

				targetDir, err = os.MkdirTemp("", "sftp-target")
				Expect(err).NotTo(HaveOccurred())

			})

			AfterEach(func() {
				os.RemoveAll(sourceDir)
				os.RemoveAll(targetDir)
			})

			It("defaults to $HOME as the remote working directory", func() {
				runInteractiveWithPassword(sftp(target), password, func(session *gexec.Session, ptyMaster *os.File) {
					io.Copy(ptyMaster, strings.NewReader("pwd\n"))
					Eventually(session.Buffer()).Should(gbytes.Say("working directory: /home/vcap"))
					ptyMaster.Write([]byte("exit\n"))
					Eventually(session).Should(gexec.Exit())
				})
			})

			It("can send and receive files", func() {
				insession := runInteractiveWithPassword(sftp(target), password,
					func(session *gexec.Session, ptyMaster *os.File) {
						input := &bytes.Buffer{}
						input.WriteString("mkdir files\n")
						input.WriteString("cd files\n")
						input.WriteString("lcd " + sourceDir + "\n")
						input.WriteString("put " + generatedFileName + "\n")
						input.WriteString("lcd " + targetDir + "\n")
						input.WriteString("get " + generatedFileName + "\n")

						io.Copy(ptyMaster, input)
						ptyMaster.Write([]byte("exit\n"))
						Eventually(session).Should(gexec.Exit())
					})

				Eventually(insession).Should(gexec.Exit())
				compareDir(sourceDir, targetDir)

			})
		})

	})

	Context("{DOCKER} in a bare-bones docker image with /bin/sh", func() {
		BeforeEach(func() {
			user = "root"
			rootfs = config.GraceBusyboxImageURL
			startTimeout = dockerTimeout
			gracePath = "/grace"
		})

		It("runs an ssh command", func() {
			cmd := ssh(target, "/bin/env")

			session := runWithPassword(cmd, password)

			Eventually(session).Should(gexec.Exit(0))
			Expect(session).To(gbytes.Say("USER=" + user))
			Expect(session).To(gbytes.Say("CUMBERBUND=cummerbund"))
		})

		It("forwards ports", func() {
			cmd := sshTunnelTo(target, 22345, 8888)

			session := runInteractiveWithPassword(cmd, password, func(session *gexec.Session, _ *os.File) {
				Eventually(session.Err).Should(gbytes.Say("Warning: Permanently added"))

				resp, err := http.Get("http://127.0.0.1:22345/index")
				Expect(err).NotTo(HaveOccurred())

				defer resp.Body.Close()

				payload, err := io.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(payload)).To(Equal("inconceivable!"))

				session.Interrupt()
			})

			Eventually(session).Should(gexec.Exit())
		})
	})
})

func runWithPassword(cmd *exec.Cmd, password string) *gexec.Session {
	return runInteractiveWithPassword(cmd, password, func(session *gexec.Session, _ *os.File) {
		Eventually(session).Should(gexec.Exit())
	})
}

func runInteractiveWithPassword(cmd *exec.Cmd, password string, actions func(*gexec.Session, *os.File)) *gexec.Session {
	passwordInput := password + "\n"

	// Create pty pseudo-terminal so that ptyClient can input password when
	// prompted
	ptyClient, ptyServer, err := pty.Open()
	Expect(err).NotTo(HaveOccurred())
	defer ptyClient.Close()

	cmd.Stdin = ptyServer
	cmd.Stdout = ptyServer
	cmd.Stderr = ptyServer

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setctty: true,
		Setsid:  true,
	}

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	// Close our open reference to ptyServer so that PTYClient recieves EOF
	ptyServer.Close()

	sendPassword(ptyClient, passwordInput)

	done := make(chan struct{})
	go func() {
		io.Copy(GinkgoWriter, ptyClient)
		close(done)
	}()

	actions(session, ptyClient)
	Eventually(done).Should(BeClosed())
	return session
}

func sendPassword(pty *os.File, password string) {
	passwordPrompt := []byte("password: ")

	b := make([]byte, 1)
	buf := []byte{}
	done := make(chan struct{})

	go func() {
		defer GinkgoRecover()
		for {
			n, err := pty.Read(b)
			Expect(n).To(Equal(1))
			Expect(err).NotTo(HaveOccurred())
			buf = append(buf, b[0])
			if bytes.HasSuffix(buf, passwordPrompt) {
				break
			}
		}
		n, err := pty.Write([]byte(password))
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(len(password)))

		close(done)
	}()

	Eventually(done).Should(BeClosed())
}

func compareDir(actualDir, expectedDir string) {
	actualDirInfo, err := os.Stat(actualDir)
	Expect(err).NotTo(HaveOccurred())

	expectedDirInfo, err := os.Stat(expectedDir)
	Expect(err).NotTo(HaveOccurred())

	Expect(actualDirInfo.Mode()).To(Equal(expectedDirInfo.Mode()))

	actualFiles, err := os.ReadDir(actualDir)
	Expect(err).NotTo(HaveOccurred())

	expectedFiles, err := os.ReadDir(actualDir)
	Expect(err).NotTo(HaveOccurred())

	Expect(len(actualFiles)).To(Equal(len(expectedFiles)))
	for i, actualFile := range actualFiles {
		expectedFile := expectedFiles[i]
		if actualFile.IsDir() {
			compareDir(filepath.Join(actualDir, actualFile.Name()), filepath.Join(expectedDir, expectedFile.Name()))
		} else {
			compareFile(filepath.Join(actualDir, actualFile.Name()), filepath.Join(expectedDir, expectedFile.Name()))
		}
	}
}

func compareFile(actualFile, expectedFile string) {
	actualFileInfo, err := os.Stat(actualFile)
	Expect(err).NotTo(HaveOccurred())

	expectedFileInfo, err := os.Stat(expectedFile)
	Expect(err).NotTo(HaveOccurred())

	Expect(actualFileInfo.Mode()).To(Equal(expectedFileInfo.Mode()))
	Expect(actualFileInfo.Size()).To(Equal(expectedFileInfo.Size()))

	actualContents, err := os.ReadFile(actualFile)
	Expect(err).NotTo(HaveOccurred())

	expectedContents, err := os.ReadFile(expectedFile)
	Expect(err).NotTo(HaveOccurred())

	Expect(actualContents).To(Equal(expectedContents))
}
