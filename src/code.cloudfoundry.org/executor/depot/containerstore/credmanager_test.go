package containerstore_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"time"

	bbsmodels "code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/containerstore"
	"code.cloudfoundry.org/executor/depot/containerstore/containerstorefakes"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("CredManager", func() {
	var (
		credManager           containerstore.CredManager
		validityPeriod        time.Duration
		CaCert                *x509.Certificate
		privateKey            *rsa.PrivateKey
		reader                io.Reader
		credManagerKeyGen     func(io.Reader, int) (*rsa.PrivateKey, error)
		logger                lager.Logger
		clock                 *fakeclock.FakeClock
		fakeMetronClient      *mfakes.FakeIngressClient
		fakeCredHandler       *containerstorefakes.FakeCredentialHandler
		containerInfoProvider *containerstorefakes.FakeContainerInfoProvider
	)

	BeforeEach(func() {

		SetDefaultEventuallyTimeout(10 * time.Second)

		validityPeriod = time.Minute
		fakeMetronClient = &mfakes.FakeIngressClient{}

		fakeCredHandler = &containerstorefakes.FakeCredentialHandler{}

		// we have seen private key generation take a long time in CI, the
		// suspicion is that `getrandom` is getting slower with the increased
		// number of certs we create on the system. This is an experiment to see if
		// using math/rand in the tests will make things less flaky. We are also
		// suspicious that this is affecting cacheddownloader TLS tests
		reader = rand.Reader
		credManagerKeyGen = rsa.GenerateKey

		logger = lagertest.NewTestLogger("credmanager")
		// Truncate and set to UTC time because of parsing time from certificate
		// and only has second granularity
		clock = fakeclock.NewFakeClock(time.Now().UTC().Truncate(time.Second))

		CaCert, privateKey = createIntermediateCert()
		containerInfoProvider = &containerstorefakes.FakeContainerInfoProvider{}
	})

	JustBeforeEach(func() {
		credManager = containerstore.NewCredManager(
			logger,
			fakeMetronClient,
			validityPeriod,
			reader,
			clock,
			CaCert,
			privateKey,
			[]containerstore.CredentialHandler{fakeCredHandler},
			containerstore.WithKeyGenerator(credManagerKeyGen),
		)
	})

	Context("NoopCredManager", func() {
		It("returns a dummy runner", func() {
			container := executor.Container{
				Guid:       fmt.Sprintf("container-guid-%d", GinkgoParallelProcess()),
				InternalIP: "127.0.0.1",
				RunInfo: executor.RunInfo{CertificateProperties: executor.CertificateProperties{
					OrganizationalUnit: []string{"app:iamthelizardking"}},
				},
			}
			containerInfoProvider.InfoReturns(container)

			runner := containerstore.NewNoopCredManager().Runner(logger, containerInfoProvider, make(<-chan struct{}, 1))
			process := ifrit.Background(runner)
			Eventually(process.Ready()).Should(BeClosed())
			Consistently(process.Wait()).ShouldNot(Receive())
			process.Signal(os.Interrupt)
			Eventually(process.Wait()).Should(Receive())
		})
	})

	Context("RemoveCredDir", func() {
		var (
			fakeCredHandler1, fakeCredHandler2 *containerstorefakes.FakeCredentialHandler
		)

		JustBeforeEach(func() {
			fakeCredHandler1 = &containerstorefakes.FakeCredentialHandler{}
			fakeCredHandler2 = &containerstorefakes.FakeCredentialHandler{}

			credManager = containerstore.NewCredManager(
				logger,
				fakeMetronClient,
				validityPeriod,
				reader,
				clock,
				CaCert,
				privateKey,
				[]containerstore.CredentialHandler{fakeCredHandler1, fakeCredHandler2},
			)
		})

		It("calls the handlers RemoveDir", func() {
			container := executor.Container{Guid: "guid"}
			credManager.RemoveCredDir(logger, container)
			Expect(fakeCredHandler1.RemoveDirCallCount()).To(Equal(1))
			Expect(fakeCredHandler2.RemoveDirCallCount()).To(Equal(1))
			_, actualContainer := fakeCredHandler1.RemoveDirArgsForCall(0)
			Expect(actualContainer).To(Equal(container))
			_, actualContainer = fakeCredHandler2.RemoveDirArgsForCall(0)
			Expect(actualContainer).To(Equal(container))
		})

		It("if the first handler returned an error continue to call RemoveDir on other handlers", func() {
			fakeCredHandler1.RemoveDirReturns(errors.New("boooom!"))
			credManager.RemoveCredDir(logger, executor.Container{Guid: "guid"})

			Expect(fakeCredHandler1.RemoveDirCallCount()).Should(Equal(1))
			Expect(fakeCredHandler2.RemoveDirCallCount()).Should(Equal(1))
		})

		It("returns an error if one of the handlers returned an error", func() {
			fakeCredHandler2.RemoveDirReturns(errors.New("boooom!"))
			err := credManager.RemoveCredDir(logger, executor.Container{Guid: "guid"})

			Expect(err.Error()).To(Equal("boooom!"))
		})

		It("returns nil if there are no errors", func() {
			fakeCredHandler2.RemoveDirReturns(nil)
			err := credManager.RemoveCredDir(logger, executor.Container{Guid: "guid"})

			Expect(err).To(BeNil())
		})
	})

	Context("CreateCredDir", func() {
		var (
			fakeCredHandler1, fakeCredHandler2 *containerstorefakes.FakeCredentialHandler
		)

		JustBeforeEach(func() {
			fakeCredHandler1 = &containerstorefakes.FakeCredentialHandler{}
			fakeCredHandler2 = &containerstorefakes.FakeCredentialHandler{}

			credManager = containerstore.NewCredManager(
				logger,
				fakeMetronClient,
				validityPeriod,
				reader,
				clock,
				CaCert,
				privateKey,
				[]containerstore.CredentialHandler{fakeCredHandler1, fakeCredHandler2},
			)
		})

		It("calls the handlers CreateDir", func() {
			container := executor.Container{Guid: "guid"}
			credManager.CreateCredDir(logger, container)
			Expect(fakeCredHandler1.CreateDirCallCount()).To(Equal(1))
			Expect(fakeCredHandler2.CreateDirCallCount()).To(Equal(1))
			_, actualContainer := fakeCredHandler1.CreateDirArgsForCall(0)
			Expect(actualContainer).To(Equal(container))
			_, actualContainer = fakeCredHandler2.CreateDirArgsForCall(0)
			Expect(actualContainer).To(Equal(container))
		})

		It("collects the bind mounts from all handlers", func() {
			mount1 := garden.BindMount{
				SrcPath: "/src/path1",
				DstPath: "/dst/path1",
			}
			mount2 := garden.BindMount{
				SrcPath: "/src/path2",
				DstPath: "/dst/path2",
			}
			fakeCredHandler1.CreateDirReturns([]garden.BindMount{mount1}, nil, nil)
			fakeCredHandler2.CreateDirReturns([]garden.BindMount{mount2}, nil, nil)

			mounts, _, _ := credManager.CreateCredDir(logger, executor.Container{Guid: "guid"})

			Expect(mounts).To(ConsistOf(mount1, mount2))
		})

		It("collects all environment variables", func() {
			env1 := executor.EnvironmentVariable{Name: "env1", Value: "val1"}
			env2 := executor.EnvironmentVariable{Name: "env2", Value: "val2"}
			fakeCredHandler1.CreateDirReturns(nil, []executor.EnvironmentVariable{env1}, nil)
			fakeCredHandler2.CreateDirReturns(nil, []executor.EnvironmentVariable{env2}, nil)

			_, envs, _ := credManager.CreateCredDir(logger, executor.Container{Guid: "guid"})

			Expect(envs).To(ConsistOf(env1, env2))
		})

		It("returns an error if one of the handlers returned an error", func() {
			fakeCredHandler2.CreateDirReturns(nil, nil, errors.New("boooom!"))
			_, _, err := credManager.CreateCredDir(logger, executor.Container{Guid: "guid"})

			Expect(err).To(MatchError("boooom!"))
		})
	})

	Context("WithCreds", func() {
		var (
			container executor.Container
		)

		BeforeEach(func() {
			container = executor.Container{
				Guid:       fmt.Sprintf("container-guid-%d", GinkgoParallelProcess()),
				InternalIP: "127.0.0.1",
				RunInfo: executor.RunInfo{
					InternalRoutes: bbsmodels.InternalRoutes{
						{Hostname: "a.apps.internal"},
						{Hostname: "b.apps.internal"},
					},
					CertificateProperties: executor.CertificateProperties{
						OrganizationalUnit: []string{"app:iamthelizardking"},
					},
				},
			}
		})

		Context("Runner", func() {
			var (
				containerProcess  ifrit.Process
				regenerateCertsCh chan struct{}
			)

			JustBeforeEach(func() {
				var runner ifrit.Runner
				regenerateCertsCh = make(chan struct{}, 1)
				containerInfoProvider.InfoReturns(container)
				runner = credManager.Runner(logger, containerInfoProvider, regenerateCertsCh)
				containerProcess = ifrit.Background(runner)
			})

			AfterEach(func() {
				containerProcess.Signal(os.Interrupt)
				Eventually(containerProcess.Wait()).Should(Receive())
			})

			// Since Go 1.26, crypto/rsa.GenerateKey ignores the passed-in rand
			// reader and uses the system's internal CSPRNG, so we inject a
			// failing key generator function to simulate this error path.
			Context("when generating private key fails", func() {
				BeforeEach(func() {
					credManagerKeyGen = func(_ io.Reader, _ int) (*rsa.PrivateKey, error) {
						return nil, io.EOF
					}
				})

				It("returns an error", func() {
					var err error
					Eventually(containerProcess.Wait()).Should(Receive(&err))
					Expect(err).To(MatchError("EOF"))
				})

				It("emits metrics around failed credential creation", func() {
					var err error
					Eventually(containerProcess.Wait()).Should(Receive(&err))
					Expect(err).To(MatchError("EOF"))

					Expect(fakeMetronClient.IncrementCounterCallCount()).To(Equal(1))
					metric := fakeMetronClient.IncrementCounterArgsForCall(0)
					Expect(metric).To(Equal("CredCreationFailedCount"))
				})
			})

			Context("when the handler returns an error", func() {
				BeforeEach(func() {
					fakeCredHandler.UpdateReturns(errors.New("boooom!"))
				})

				It("the runner exits", func() {
					Eventually(containerProcess.Wait()).Should(Receive(MatchError("boooom!")))
				})
			})

			Context("when runner becomes ready", func() {
				AfterEach(func() {
					containerProcess.Signal(os.Interrupt)
				})

				JustBeforeEach(func() {
					Eventually(containerProcess.Ready()).Should(BeClosed())
				})

				It("emits metrics on successful creation", func() {
					Expect(fakeMetronClient.IncrementCounterCallCount()).To(Equal(2))
					metric := fakeMetronClient.IncrementCounterArgsForCall(0)
					Expect(metric).To(Equal("CredCreationSucceededCount"))
					metric = fakeMetronClient.IncrementCounterArgsForCall(1)
					Expect(metric).To(Equal("C2CCredCreationSucceededCount"))

					Expect(fakeMetronClient.SendDurationCallCount()).To(Equal(2))
					metric, value, _ := fakeMetronClient.SendDurationArgsForCall(0)
					Expect(metric).To(Equal("CredCreationSucceededDuration"))
					Expect(value).To(BeNumerically(">=", 0))
					metric, value, _ = fakeMetronClient.SendDurationArgsForCall(1)
					Expect(metric).To(Equal("C2CCredCreationSucceededDuration"))
					Expect(value).To(BeNumerically(">=", 0))
				})

				It("calls the handler with the initiali credentials", func() {
					Eventually(fakeCredHandler.UpdateCallCount).Should(Equal(1))
				})

				Context("when the certificate is about to expire", func() {
					var (
						credsBefore containerstore.Credentials
					)

					JustBeforeEach(func() {
						Eventually(fakeCredHandler.UpdateCallCount).Should(Equal(1))

						Eventually(containerProcess.Ready()).Should(BeClosed())
						credsBefore, _ = fakeCredHandler.UpdateArgsForCall(0)
					})

					testCredentialRotation := func(dur time.Duration) {
						callCount := fakeCredHandler.UpdateCallCount()

						var actualContainer executor.Container
						credsBefore, actualContainer = fakeCredHandler.UpdateArgsForCall(callCount - 1)

						Expect(actualContainer).To(Equal(container))

						idCertBefore, _ := parseCert(credsBefore.InstanceIdentityCredential)
						c2cCertBefore, _ := parseCert(credsBefore.C2CCredential)
						Expect(idCertBefore.NotAfter).To(Equal(c2cCertBefore.NotAfter))
						increment := idCertBefore.NotAfter.Add(-dur).Sub(clock.Now())
						Expect(increment).To(BeNumerically(">", 0))
						clock.WaitForWatcherAndIncrement(increment)

						Eventually(fakeCredHandler.UpdateCallCount).Should(Equal(callCount + 1))

						cred, actualContainer := fakeCredHandler.UpdateArgsForCall(callCount)
						Expect(actualContainer).To(Equal(container))

						Expect(cred.InstanceIdentityCredential.Cert).NotTo(Equal(credsBefore.InstanceIdentityCredential.Cert))
						Expect(cred.InstanceIdentityCredential.Key).NotTo(Equal(credsBefore.InstanceIdentityCredential.Key))

						Expect(cred.C2CCredential.Cert).NotTo(Equal(credsBefore.C2CCredential.Cert))
						Expect(cred.C2CCredential.Key).NotTo(Equal(credsBefore.C2CCredential.Key))

						idCert, _ := parseCert(cred.InstanceIdentityCredential)
						Expect(idCert.SerialNumber).NotTo(Equal(idCertBefore.SerialNumber))

						c2cCert, _ := parseCert(cred.C2CCredential)
						Expect(c2cCert.SerialNumber).NotTo(Equal(c2cCertBefore.SerialNumber))
					}

					testNoCredentialRotation := func(dur time.Duration) {
						callCount := fakeCredHandler.UpdateCallCount()
						credsBefore, _ = fakeCredHandler.UpdateArgsForCall(callCount - 1)

						idCertBefore, _ := parseCert(credsBefore.InstanceIdentityCredential)
						c2cCertBefore, _ := parseCert(credsBefore.C2CCredential)
						Expect(idCertBefore.NotAfter).To(Equal(c2cCertBefore.NotAfter))
						increment := idCertBefore.NotAfter.Add(-dur).Sub(clock.Now())
						Expect(increment).To(BeNumerically(">", 0))
						clock.WaitForWatcherAndIncrement(increment)

						Consistently(fakeCredHandler.UpdateCallCount).Should(Equal(callCount))
					}

					Context("when the handler returns an error", func() {
						JustBeforeEach(func() {
							fakeCredHandler.UpdateReturnsOnCall(1, errors.New("boooom!"))
							idCertBefore, _ := parseCert(credsBefore.InstanceIdentityCredential)
							increment := idCertBefore.NotAfter.Sub(clock.Now())
							clock.WaitForWatcherAndIncrement(increment)
						})

						It("the runner exits", func() {
							Eventually(containerProcess.Wait()).Should(Receive(MatchError("boooom!")))
						})
					})

					Context("when it recieves a message on the regenerateCertsCh", func() {
						It("regenerates a c2c certificate", func() {
							Expect(fakeCredHandler.UpdateCallCount()).To(Equal(1))
							cred, _ := fakeCredHandler.UpdateArgsForCall(0)
							initC2cCert, _ := parseCert(cred.C2CCredential)

							regenerateCertsCh <- struct{}{}

							Eventually(fakeCredHandler.UpdateCallCount).Should(Equal(2))
							cred, _ = fakeCredHandler.UpdateArgsForCall(1)

							By("regenerating c2c certificate")
							finalC2cCert, _ := parseCert(cred.C2CCredential)
							Expect(initC2cCert.SerialNumber).ToNot(Equal(finalC2cCert.SerialNumber))

							By("not regenerating instance identity certificate")
							Expect(cred.InstanceIdentityCredential.IsEmpty()).To(BeTrue())
						})

						Context("when internal routes were updated", func() {
							BeforeEach(func() {
								container = executor.Container{
									Guid:       fmt.Sprintf("container-guid-%d", GinkgoParallelProcess()),
									InternalIP: "127.0.0.1",
									RunInfo: executor.RunInfo{
										InternalRoutes: bbsmodels.InternalRoutes{
											{Hostname: "a.apps.internal"},
											{Hostname: "b.apps.internal"},
										},
										CertificateProperties: executor.CertificateProperties{
											OrganizationalUnit: []string{"app:iamthelizardking"},
										},
									},
								}
							})

							It("regenerates a c2c certificate", func() {
								Expect(fakeCredHandler.UpdateCallCount()).To(Equal(1))
								cred, _ := fakeCredHandler.UpdateArgsForCall(0)
								c2cCert, _ := parseCert(cred.C2CCredential)
								containerGuid := fmt.Sprintf("container-guid-%d", GinkgoParallelProcess())
								Expect(c2cCert.DNSNames).To(ConsistOf(containerGuid, "a.apps.internal", "b.apps.internal"))

								container.RunInfo.InternalRoutes = bbsmodels.InternalRoutes{
									{Hostname: "a.apps.internal"},
									{Hostname: "c.apps.internal"},
								}
								containerInfoProvider.InfoReturns(container)

								regenerateCertsCh <- struct{}{}

								getDNSNames := func() []string {
									cred, _ = fakeCredHandler.UpdateArgsForCall(fakeCredHandler.UpdateCallCount() - 1)
									c2cCert, _ = parseCert(cred.C2CCredential)
									return c2cCert.DNSNames
								}

								Eventually(getDNSNames).Should(ConsistOf(containerGuid, "a.apps.internal", "c.apps.internal"))
							})
						})
					})

					Context("when the certificate validity is less than 4 hours", func() {
						BeforeEach(func() {

							By("Starting #1")
							validityPeriod = time.Minute
						})

						Context("when 15 seconds prior to expiry", func() {
							It("does not rotate the credentials", func() {
								By("Starting #2")
								testNoCredentialRotation(15 * time.Second)
							})
						})

						Context("when 5 seconds prior to expiry", func() {
							It("rotates the certificates", func() {
								By("Starting #3")
								testCredentialRotation(5 * time.Second)
							})

							It("emits metrics on successful creation", func() {
								By("Starting #4")
								cert, _ := parseCert(credsBefore.InstanceIdentityCredential)
								increment := cert.NotAfter.Add(-5 * time.Second).Sub(clock.Now())
								Expect(increment).To(BeNumerically(">", 0))
								clock.WaitForWatcherAndIncrement(increment)

								Eventually(fakeMetronClient.IncrementCounterCallCount).Should(Equal(4))
								By("Starting #5")
								metric := fakeMetronClient.IncrementCounterArgsForCall(2)
								Expect(metric).To(Equal("CredCreationSucceededCount"))
								metric = fakeMetronClient.IncrementCounterArgsForCall(3)
								Expect(metric).To(Equal("C2CCredCreationSucceededCount"))

								Expect(fakeMetronClient.SendDurationCallCount()).To(Equal(4))
								metric, value, _ := fakeMetronClient.SendDurationArgsForCall(2)
								Expect(metric).To(Equal("CredCreationSucceededDuration"))
								Expect(value).To(BeNumerically(">=", 0))
								metric, value, _ = fakeMetronClient.SendDurationArgsForCall(3)
								Expect(metric).To(Equal("C2CCredCreationSucceededDuration"))
								Expect(value).To(BeNumerically(">=", 0))
							})
						})

						// test timer reset logic
						Context("after credential rotation", func() {
							JustBeforeEach(func() {
								testCredentialRotation(5 * time.Second)
							})

							Context("when 5 seconds prior to expiry", func() {
								It("rotates the certs", func() {
									testCredentialRotation(5 * time.Second)
								})
							})

							Context("when 15 seconds prior to expiry", func() {
								It("does not rotate the credentials", func() {
									testNoCredentialRotation(15 * time.Second)
								})
							})
						})
					})

					Context("when certificate validity is longer than 4 hours", func() {
						BeforeEach(func() {
							validityPeriod = 24 * time.Hour
						})

						Context("when 90 minutes prior to expiry", func() {
							It("does not rotate the credentials", func() {
								testNoCredentialRotation(90 * time.Minute)
							})
						})

						Context("when 30 minutes prior to expiry", func() {
							It("rotates the certs", func() {
								testCredentialRotation(30 * time.Minute)
							})
						})
					})
				})

				Describe("the certificate", func() {
					var (
						cert *x509.Certificate
						rest []byte
					)

					testCertificateFields := func() {
						By("has all required usages in the KU & EKU fields")
						Expect(cert.ExtKeyUsage).To(ContainElement(x509.ExtKeyUsageClientAuth))
						Expect(cert.ExtKeyUsage).To(ContainElement(x509.ExtKeyUsageServerAuth))
						Expect(cert.KeyUsage).To(Equal(x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageKeyAgreement))

						By("has the expected key length")
						rsaKey, ok := cert.PublicKey.(*rsa.PublicKey)
						Expect(ok).To(BeTrue(), "expected RSA public key")
						Expect(rsaKey.N.BitLen()).To(Equal(containerstore.RSAPrivateKeySize))

						By("signed by the rep intermediate CA")
						CaCertPool := x509.NewCertPool()
						CaCertPool.AddCert(CaCert)
						verifyOpts := x509.VerifyOptions{Roots: CaCertPool}
						Expect(cert.CheckSignatureFrom(CaCert)).To(Succeed())
						_, err := cert.Verify(verifyOpts)
						Expect(err).NotTo(HaveOccurred())

						By("common name should be set to the container guid")
						Expect(cert.Subject.CommonName).To(Equal(container.Guid))

						By("DNS SAN should be set to the container guid")
						Expect(cert.DNSNames).To(ContainElement(container.Guid))

						By("expires in after the configured validity period")
						Expect(cert.NotAfter).To(Equal(clock.Now().Add(validityPeriod)))

						By("not before is set to current timestamp")
						Expect(cert.NotBefore).To(Equal(clock.Now()))

						By("has the rep intermediate CA")
						block, rest := pem.Decode(rest)
						Expect(block).NotTo(BeNil())
						Expect(rest).To(BeEmpty())
						Expect(block.Type).To(Equal("CERTIFICATE"))
						certs, err := x509.ParseCertificates(block.Bytes)
						Expect(err).NotTo(HaveOccurred())
						Expect(certs).To(HaveLen(1))
						Expect(certs[0]).To(Equal(CaCert))

						By("has the app guid in the subject's organizational units")
						Expect(cert.Subject.OrganizationalUnit).To(ContainElement("app:iamthelizardking"))
					}

					Describe("Instance identity certificate", func() {
						JustBeforeEach(func() {
							Eventually(containerProcess.Ready()).Should(BeClosed())

							Eventually(fakeCredHandler.UpdateCallCount).Should(Equal(1))
							creds, actualContainer := fakeCredHandler.UpdateArgsForCall(0)
							Expect(actualContainer).To(Equal(container))
							cert, rest = parseCert(creds.InstanceIdentityCredential)
						})

						Context("when the container doesn't have an internal ip", func() {
							Context("when the container has an external IP", func() {
								BeforeEach(func() {
									container = executor.Container{
										Guid:       fmt.Sprintf("container-guid-%d", GinkgoParallelProcess()),
										InternalIP: "",
										ExternalIP: "54.23.123.234",
										RunInfo: executor.RunInfo{CertificateProperties: executor.CertificateProperties{
											OrganizationalUnit: []string{"app:iamthelizardking"}},
										},
									}
								})

								It("has the external ip", func() {
									ip := net.ParseIP(container.ExternalIP)
									Expect(cert.IPAddresses).To(ContainElement(ip.To4()))
								})

								It("does not have the empty internal ip", func() {
									ip := net.ParseIP(container.InternalIP)
									Expect(cert.IPAddresses).NotTo(ContainElement(ip.To4()))
								})
							})

							Context("when the container doesn't have an external ip", func() {
								BeforeEach(func() {
									container = executor.Container{
										Guid:       fmt.Sprintf("container-guid-%d", GinkgoParallelProcess()),
										InternalIP: "",
										ExternalIP: "",
										RunInfo: executor.RunInfo{CertificateProperties: executor.CertificateProperties{
											OrganizationalUnit: []string{"app:iamthelizardking"}},
										},
									}
								})

								It("has no SubjectAltName", func() {
									Expect(cert.IPAddresses).To(BeEmpty())
								})

							})
						})

						It("has the container ip", func() {
							ip := net.ParseIP(container.InternalIP)
							Expect(cert.IPAddresses).To(ContainElement(ip.To4()))
						})

						It("has the correct fields", func() {
							testCertificateFields()
						})
					})

					Describe("C2C certificate", func() {
						JustBeforeEach(func() {
							Eventually(containerProcess.Ready()).Should(BeClosed())

							Eventually(fakeCredHandler.UpdateCallCount).Should(Equal(1))
							creds, actualContainer := fakeCredHandler.UpdateArgsForCall(0)
							Expect(actualContainer).To(Equal(container))
							cert, rest = parseCert(creds.C2CCredential)
						})

						It("has the internal routes in SAN", func() {
							Expect(cert.DNSNames).To(ConsistOf(container.Guid, "a.apps.internal", "b.apps.internal"))
						})

						It("has the correct fields", func() {
							testCertificateFields()
						})
					})
				})
			})

			Context("when signalled", func() {
				JustBeforeEach(func() {
					Eventually(containerProcess.Ready()).Should(BeClosed())
					containerProcess.Signal(os.Interrupt)
				})

				// deleting the directory this early can cause failures on windows 1803, see #156406881
				It("does not call RemoveDir on the handlers", func() {
					Eventually(fakeCredHandler.RemoveDirCallCount).Should(BeZero())
				})

				It("Generates an invalid cert and sends the invalid cert on the cred channel", func() {
					Eventually(fakeCredHandler.CloseCallCount).Should(Equal(1))

					creds, actualContainer := fakeCredHandler.CloseArgsForCall(0)
					Expect(actualContainer).To(Equal(container))

					block, _ := pem.Decode([]byte(creds.InstanceIdentityCredential.Cert))
					Expect(block).NotTo(BeNil())
					certs, err := x509.ParseCertificates(block.Bytes)
					Expect(err).NotTo(HaveOccurred())
					cert := certs[0]

					Expect(cert.Subject.CommonName).To(BeEmpty())

					block, _ = pem.Decode([]byte(creds.C2CCredential.Cert))
					Expect(block).NotTo(BeNil())
					certs, err = x509.ParseCertificates(block.Bytes)
					Expect(err).NotTo(HaveOccurred())
					cert = certs[0]

					Expect(cert.Subject.CommonName).To(BeEmpty())
				})
			})
		})
	})
})

func createIntermediateCert() (*x509.Certificate, *rsa.PrivateKey) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())

	template := &x509.Certificate{
		IsCA:                  true,
		BasicConstraintsValid: true,
		SerialNumber:          big.NewInt(1),
		NotAfter:              time.Now().Add(36 * time.Hour),
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, privateKey.Public(), privateKey)
	Expect(err).NotTo(HaveOccurred())

	certs, err := x509.ParseCertificates(certBytes)
	Expect(err).NotTo(HaveOccurred())
	Expect(certs).To(HaveLen(1))
	return certs[0], privateKey
}

func parseCert(cred containerstore.Credential) (*x509.Certificate, []byte) {
	var block *pem.Block
	var rest []byte
	block, rest = pem.Decode([]byte(cred.Cert))
	Expect(block).NotTo(BeNil())
	Expect(block.Type).To(Equal("CERTIFICATE"))
	certs, err := x509.ParseCertificates(block.Bytes)
	Expect(err).NotTo(HaveOccurred())
	return certs[0], rest
}
