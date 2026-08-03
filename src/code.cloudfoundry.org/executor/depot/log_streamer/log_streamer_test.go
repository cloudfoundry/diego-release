package log_streamer_test

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/bbs/models"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/executor/depot/log_streamer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

var _ = Describe("LogStreamer", func() {
	var (
		streamer             log_streamer.LogStreamer
		fakeClient           *mfakes.FakeIngressClient
		logConfig            models.LogConfig
		maxLogLinesPerSecond int
		maxLogBytesPerSecond int64
		metricReportInterval time.Duration
	)

	guid := "the-guid"
	sourceName := "the-source-name"
	index := 11
	tags := map[string]string{
		"foo": "bar",
		"biz": "baz",
	}

	BeforeEach(func() {
		maxLogLinesPerSecond, maxLogBytesPerSecond = 9999, -1
		metricReportInterval = 5 * time.Minute
		fakeClient = &mfakes.FakeIngressClient{}
		logConfig = models.LogConfig{Guid: guid, SourceName: sourceName, Index: index, Tags: tags}
		streamer = log_streamer.New(logConfig, fakeClient, maxLogLinesPerSecond, maxLogBytesPerSecond, metricReportInterval)
	})

	Context("when told to emit", func() {
		Context("when given two messages under the line limit", func() {
			BeforeEach(func() {
				_, _ = fmt.Fprintln(streamer.Stdout(), "this is a log")
				_, _ = fmt.Fprintln(streamer.Stdout(), "this is another log")
			})

			It("should emit both messages", func() {
				Expect(fakeClient.SendAppLogCallCount()).To(Equal(2))

				message, sn, tags := fakeClient.SendAppLogArgsForCall(0)
				Expect(tags["source_id"]).To(Equal(guid))
				Expect(sn).To(Equal(sourceName))
				Expect(message).To(Equal("this is a log"))
				Expect(tags["instance_id"]).To(Equal("11"))
				Expect(tags["foo"]).To(Equal("bar"))
				Expect(tags["biz"]).To(Equal("baz"))

				message, sn, tags = fakeClient.SendAppLogArgsForCall(1)
				Expect(tags["source_id"]).To(Equal(guid))
				Expect(sn).To(Equal(sourceName))
				Expect(message).To(Equal("this is another log"))
				Expect(tags["instance_id"]).To(Equal("11"))
				Expect(tags["foo"]).To(Equal("bar"))
				Expect(tags["biz"]).To(Equal("baz"))
			})
		})

		Context("when given a log byte size arrival rate exceeds the maximum allowed", func() {
			Context("rate limit is applied at a lower threshold", func() {
				var logLine string
				BeforeEach(func() {
					maxLogBytesPerSecond = 1000
					streamer = log_streamer.New(logConfig, fakeClient, maxLogLinesPerSecond, maxLogBytesPerSecond, metricReportInterval)

					logLine := strings.Repeat("a", int(maxLogBytesPerSecond/2))
					for i := 0; i < 3; i++ {
						fmt.Fprintf(streamer.Stdout(), logLine+"\n\n\n\n", i)
					}
				})

				It("should rate limit the messages", func() {
					Eventually(func() int {
						printedMsgs := 0
						for i := 0; i < fakeClient.SendAppLogCallCount(); i++ {
							msg, _, _ := fakeClient.SendAppLogArgsForCall(i)
							if strings.Contains(msg, logLine) {
								printedMsgs++
							}
						}
						return printedMsgs
					}, 1*time.Second).Should(Equal(2))

					Eventually(fakeClient.SendAppLogCallCount, 1*time.Second).Should(BeNumerically(">=", 2))
					calls := fakeClient.SendAppLogCallCount()
					var args []string
					for i := 0; i < calls; i++ {
						msg, _, _ := fakeClient.SendAppLogArgsForCall(i)
						args = append(args, msg)
					}

					var expectedArgs []types.GomegaMatcher
					expectedArgs = append(expectedArgs, ContainSubstring(logLine))
					expectedArgs = append(expectedArgs, ContainSubstring(fmt.Sprintf("app instance exceeded log rate limit (%d bytes/sec)", maxLogBytesPerSecond)))

					Expect(args).To(ConsistOf(expectedArgs))
				})
			})
		})

		Context("Max bytes per second is zero", func() {
			BeforeEach(func() {
				maxLogBytesPerSecond = 0
				streamer = log_streamer.New(logConfig, fakeClient, maxLogLinesPerSecond, maxLogBytesPerSecond, metricReportInterval)

				logLine := strings.Repeat("a", 20)
				for i := 0; i < 3; i++ {
					fmt.Fprintf(streamer.Stdout(), logLine+"\n", i)
				}
			})

			It("should not log", func() {
				Consistently(fakeClient.SendAppLogCallCount, 1*time.Second).Should(Equal(0))
			})
		})

		Context("Byte limit is low", func() {
			BeforeEach(func() {
				maxLogLinesPerSecond, maxLogBytesPerSecond = 1, 100
				streamer = log_streamer.New(logConfig, fakeClient, maxLogLinesPerSecond, maxLogBytesPerSecond, metricReportInterval)

				fmt.Fprintln(streamer.Stdout(), strings.Repeat("a", 1000))
				fmt.Fprintln(streamer.Stdout(), "bbbbbbb")
			})

			It("causes outages of logs", func() {
				Eventually(fakeClient.SendAppLogCallCount, 1*time.Second).Should(Equal(1))

				var messages []string
				for i := 0; i < fakeClient.SendAppLogCallCount(); i++ {
					msg, _, _ := fakeClient.SendAppLogArgsForCall(i)
					messages = append(messages, msg)
				}

				Expect(messages).To(ConsistOf(
					Equal("app instance exceeded log rate limit (100 bytes/sec)"),
				))
			})
		})

		Context("Log rate limit is low", func() {
			BeforeEach(func() {
				maxLogLinesPerSecond, maxLogBytesPerSecond = 1, 100
				streamer = log_streamer.New(logConfig, fakeClient, maxLogLinesPerSecond, maxLogBytesPerSecond, metricReportInterval)

				for i := 0; i < 10000; i++ {
					fmt.Fprintf(streamer.Stdout(), "a\n")
				}
			})

			It("skips logs when streamer is stopped", func() {
				Eventually(fakeClient.SendAppLogCallCount, 1*time.Second).Should(BeNumerically(">", 0))
				streamer.Stop()
				callCount := fakeClient.SendAppLogCallCount()
				Consistently(fakeClient.SendAppLogCallCount, 1*time.Second).Should(Equal(callCount))
			})
		})

		Context("when given a log line arrival rate exceeds the maximum allowed", func() {
			Context("rate limit is applied at a lower threshold", func() {
				BeforeEach(func() {
					maxLogLinesPerSecond = 1
					streamer = log_streamer.New(logConfig, fakeClient, maxLogLinesPerSecond, maxLogBytesPerSecond, metricReportInterval)

					for i := 0; i < maxLogLinesPerSecond*3; i++ {
						fmt.Fprintf(streamer.Stdout(), "this is log # %d\n", i)
					}
				})

				It("should rate limit the messages", func() {
					Eventually(func() int {
						printedMsgs := 0
						for i := 0; i < fakeClient.SendAppLogCallCount(); i++ {
							msg, _, _ := fakeClient.SendAppLogArgsForCall(i)
							if strings.Contains(msg, "this is log") {
								printedMsgs++
							}
						}
						return printedMsgs
					}, 1*time.Second).Should(BeNumerically("==", 1))

					Eventually(fakeClient.SendAppLogCallCount).Should(Equal(2))
					calls := fakeClient.SendAppLogCallCount()
					args := []string{}
					for i := 0; i < calls; i++ {
						msg, _, _ := fakeClient.SendAppLogArgsForCall(i)
						args = append(args, msg)
					}

					expectedArgs := []types.GomegaMatcher{}
					expectedArgs = append(expectedArgs, ContainSubstring("this is log"))
					expectedArgs = append(expectedArgs, ContainSubstring("app instance exceeded log rate limit (1 log-lines/sec) set by platform operator"))

					Expect(args).To(ConsistOf(expectedArgs))
				})

				It("increments an AppInstanceExceededLogRateLimitCount metric", func() {
					Eventually(fakeClient.IncrementCounterCallCount, 1*time.Second).Should(Equal(1))
					metricName := fakeClient.IncrementCounterArgsForCall(0)
					Expect(metricName).To(Equal("AppInstanceExceededLogRateLimitCount"))
				})

				Context("when metric was already incremented in the past metric report interval", func() {
					It("does not increment an AppInstanceExceededLogRateLimitCount metric during report interval", func() {
						Eventually(fakeClient.SendAppLogCallCount, 1*time.Second).Should(BeNumerically(">=", 2))
						Consistently(fakeClient.IncrementCounterCallCount, 1*time.Second).Should(Equal(1))
					})
				})
			})

			Context("when metric was incremented and report interval passed", func() {
				BeforeEach(func() {
					if runtime.GOOS == "windows" {
						Skip("Go 1.16 bug, waiting for a fix https://github.com/golang/go/issues/44343")
					}
					maxLogLinesPerSecond = 1
					metricReportInterval = time.Second
					streamer = log_streamer.New(logConfig, fakeClient, maxLogLinesPerSecond, maxLogBytesPerSecond, metricReportInterval)

					for i := 0; i < 3; i++ {
						fmt.Fprintf(streamer.Stdout(), "this is log # %d \n", i)
					}
				})

				It("increments an AppInstanceExceededLogRateLimitCount metric during report interval", func() {
					Consistently(fakeClient.SendAppLogCallCount, 1*time.Second).Should(BeNumerically("<", 4))
					Eventually(fakeClient.SendAppLogCallCount, 1*time.Second).Should(BeNumerically(">=", 2))
					Eventually(fakeClient.IncrementCounterCallCount, 1*time.Second).Should(Equal(1))
				})
			})

			Context("rate limit is not applied", func() {
				BeforeEach(func() {
					maxLogLinesPerSecond = 0
					streamer = log_streamer.New(logConfig, fakeClient, maxLogLinesPerSecond, maxLogBytesPerSecond, metricReportInterval)

					for i := 0; i < 20; i++ {
						fmt.Fprintf(streamer.Stdout(), "this is log # %d \n", i)
					}
				})

				It("should not rate limit the messages", func() {
					Eventually(fakeClient.SendAppLogCallCount, time.Second).Should(Equal(20))
				})
			})

			Context("rate limit is bigger than number of log lines", func() {
				BeforeEach(func() {
					maxLogLinesPerSecond = 6
					streamer = log_streamer.New(logConfig, fakeClient, maxLogLinesPerSecond, maxLogBytesPerSecond, metricReportInterval)

					for i := 0; i < 3; i++ {
						fmt.Fprintf(streamer.Stdout(), "this is log # %d \n", i)
					}
				})

				It("should not rate limit the messages", func() {
					Eventually(fakeClient.SendAppLogCallCount, time.Second).Should(Equal(3))
				})
			})
		})

		Describe("WithSource", func() {
			Context("when a new log source is provided", func() {
				var newSourceName string
				var streamerWithUpdatedSource log_streamer.LogStreamer

				BeforeEach(func() {
					newSourceName = "new-source-name"
					streamerWithUpdatedSource = streamer.WithSource(newSourceName)
				})

				It("should emit a message with the new log source", func() {
					fmt.Fprintln(streamerWithUpdatedSource.Stdout(), "this is a log")
					Expect(fakeClient.SendAppLogCallCount()).To(Equal(1))

					_, sn, _ := fakeClient.SendAppLogArgsForCall(0)

					Expect(sn).To(Equal(newSourceName))
				})

				Context("when the original streamer is updated with new tags", func() {
					It("messages are emitted with the new tags", func() {
						streamer.UpdateTags(map[string]string{"new": "tags"})

						fmt.Fprintln(streamerWithUpdatedSource.Stdout(), "this is a log")

						_, _, tags := fakeClient.SendAppLogArgsForCall(0)
						Expect(tags).To(Equal(map[string]string{"new": "tags"}))
					})
				})
			})

			Context("when no log source is provided", func() {
				BeforeEach(func() {
					streamer = streamer.WithSource("")
				})

				It("should emit a message with the existing log source", func() {
					fmt.Fprintln(streamer.Stdout(), "this is a log")

					Expect(fakeClient.SendAppLogCallCount()).To(Equal(1))

					_, sn, _ := fakeClient.SendAppLogArgsForCall(0)

					Expect(sn).To(Equal(sourceName))
				})
			})

			Context("rate limit is applied at a lower threshold", func() {
				var newStreamer log_streamer.LogStreamer

				BeforeEach(func() {
					maxLogLinesPerSecond = 1
					streamer = log_streamer.New(logConfig, fakeClient, maxLogLinesPerSecond, maxLogBytesPerSecond, metricReportInterval)

					newStreamer = streamer.WithSource("new-source-name")
				})

				It("should rate limit the messages for original and new log streamers", func() {
					for i := 0; i < maxLogLinesPerSecond*3; i++ {
						fmt.Fprintf(streamer.Stdout(), "old streamer: this is log # %d \n", i)
						fmt.Fprintf(newStreamer.Stdout(), "new streamer: this is log # %d \n", i)
					}

					Eventually(func() int {
						printedMsgs := 0
						for i := 0; i < fakeClient.SendAppLogCallCount(); i++ {
							msg, _, _ := fakeClient.SendAppLogArgsForCall(i)
							if strings.Contains(msg, "this is log") {
								printedMsgs++
							}
						}
						return printedMsgs
					}, 1*time.Second).Should(Equal(1))

					calls := fakeClient.SendAppLogCallCount()
					args := []string{}
					for i := 0; i < calls; i++ {
						msg, _, _ := fakeClient.SendAppLogArgsForCall(i)
						args = append(args, msg)
					}

					expectedArgs := []types.GomegaMatcher{}
					expectedArgs = append(expectedArgs, ContainSubstring("this is log"))
					expectedArgs = append(expectedArgs, ContainSubstring("app instance exceeded log rate limit (1 log-lines/sec) set by platform operator"))

					Expect(args).To(ConsistOf(expectedArgs))
				})

				It("increments an AppInstanceExceededLogRateLimitCount metric once per interval for both streamers", func() {
					for i := 0; i < maxLogLinesPerSecond*3; i++ {
						fmt.Fprintf(streamer.Stdout(), "old streamer: this is log # %d \n", i)
						fmt.Fprintf(newStreamer.Stdout(), "new streamer: this is log # %d \n", i)
					}

					Eventually(fakeClient.IncrementCounterCallCount, 1*time.Second).Should(Equal(1))
					metricName := fakeClient.IncrementCounterArgsForCall(0)
					Expect(metricName).To(Equal("AppInstanceExceededLogRateLimitCount"))
				})

				Context("when metric was incremented and report interval passed", func() {
					BeforeEach(func() {
						maxLogLinesPerSecond = 1
						metricReportInterval = time.Second
						streamer = log_streamer.New(logConfig, fakeClient, maxLogLinesPerSecond, maxLogBytesPerSecond, metricReportInterval)
						newStreamer = streamer.WithSource("new-source-name")
					})

					It("increments an AppInstanceExceededLogRateLimitCount metric", func() {
						for i := 0; i < maxLogLinesPerSecond*3; i++ {
							fmt.Fprintf(streamer.Stdout(), "old streamer: this is log # %d \n", i)
						}
						Eventually(fakeClient.IncrementCounterCallCount, 1*time.Second).Should(Equal(1))

						for i := 0; i < maxLogLinesPerSecond*3; i++ {
							fmt.Fprintf(newStreamer.Stdout(), "new streamer: this is log # %d \n", i)
						}
						Eventually(fakeClient.IncrementCounterCallCount, 1*time.Second).Should(BeNumerically(">=", 1))
						Expect(fakeClient.SendAppLogCallCount()).To(BeNumerically(">=", 2))
					})
				})
			})
		})

		Describe("SourceName", func() {
			It("should return the log streamer's configured source name", func() {
				Expect(streamer.SourceName()).To(Equal(sourceName))
			})
		})

		Context("when given a message with all sorts of fun newline characters", func() {
			BeforeEach(func() {
				fmt.Fprintf(streamer.Stdout(), "A\nB\rC\n\rD\r\nE\n\n\nF\r\r\rG\n\r\r\n\n\n\r")
			})

			It("should do the right thing", func() {
				Expect(fakeClient.SendAppLogCallCount()).To(Equal(7))
				for i, expectedString := range []string{"A", "B", "C", "D", "E", "F", "G"} {
					message, _, _ := fakeClient.SendAppLogArgsForCall(i)
					Expect(message).To(Equal(expectedString))
				}
			})
		})

		Context("when given a series of short messages", func() {
			BeforeEach(func() {
				fmt.Fprintf(streamer.Stdout(), "this is a log")
				fmt.Fprintf(streamer.Stdout(), " it is made of wood")
				fmt.Fprintf(streamer.Stdout(), " - and it is longer")
				fmt.Fprintf(streamer.Stdout(), "than it seems\n")
			})

			It("concatenates them, until a new-line is received, and then emits that", func() {
				Expect(fakeClient.SendAppLogCallCount()).To(Equal(1))
				message, _, _ := fakeClient.SendAppLogArgsForCall(0)
				Expect(message).To(Equal("this is a log it is made of wood - and it is longerthan it seems"))
			})
		})

		Context("when given a message with multiple new lines", func() {
			BeforeEach(func() {
				fmt.Fprintf(streamer.Stdout(), "this is a log\nand this is another\nand this one isn't done yet...")
			})

			It("should break the message up into multiple loggings", func() {
				Expect(fakeClient.SendAppLogCallCount()).To(Equal(2))

				message, _, _ := fakeClient.SendAppLogArgsForCall(0)
				Expect(message).To(Equal("this is a log"))

				message, _, _ = fakeClient.SendAppLogArgsForCall(1)
				Expect(message).To(Equal("and this is another"))
			})
		})

		Describe("message limits", func() {
			var message string
			Context("when the message is just at the emittable length", func() {
				BeforeEach(func() {
					message = strings.Repeat("7", log_streamer.MAX_MESSAGE_SIZE)
					Expect([]byte(message)).To(HaveLen(log_streamer.MAX_MESSAGE_SIZE), "Ensure that the byte representation of our message is under the limit")

					fmt.Fprintln(streamer.Stdout(), message)
				})

				It("should not break the message up and send a single messages", func() {
					Expect(fakeClient.SendAppLogCallCount()).To(Equal(1))
					ms, _, _ := fakeClient.SendAppLogArgsForCall(0)
					Expect(ms).To(Equal(message))
				})
			})

			Context("when the message exceeds the emittable length", func() {
				BeforeEach(func() {
					message = strings.Repeat("7", log_streamer.MAX_MESSAGE_SIZE)
					message += strings.Repeat("8", log_streamer.MAX_MESSAGE_SIZE)
					message += strings.Repeat("9", log_streamer.MAX_MESSAGE_SIZE)
					message += "hello\n"
					fmt.Fprint(streamer.Stdout(), message)
				})

				It("should break the message up and send multiple messages", func() {
					Expect(fakeClient.SendAppLogCallCount()).To(Equal(4))

					ms, _, _ := fakeClient.SendAppLogArgsForCall(0)
					Expect(ms).To(Equal(strings.Repeat("7", log_streamer.MAX_MESSAGE_SIZE)))
					ms, _, _ = fakeClient.SendAppLogArgsForCall(1)
					Expect(ms).To(Equal(strings.Repeat("8", log_streamer.MAX_MESSAGE_SIZE)))
					ms, _, _ = fakeClient.SendAppLogArgsForCall(2)
					Expect(ms).To(Equal(strings.Repeat("9", log_streamer.MAX_MESSAGE_SIZE)))
					ms, _, _ = fakeClient.SendAppLogArgsForCall(3)
					Expect(ms).To(Equal("hello"))
				})
			})

			Context("when having to deal with byte boundaries and long utf characters", func() {
				BeforeEach(func() {
					message = strings.Repeat("a", log_streamer.MAX_MESSAGE_SIZE-3)
					message += "\U0001F428\n"
				})

				It("should break the message up and send multiple messages without sending error runes", func() {
					fmt.Fprint(streamer.Stdout(), message)
					Expect(fakeClient.SendAppLogCallCount()).To(Equal(2))

					ms, _, _ := fakeClient.SendAppLogArgsForCall(0)
					Expect(ms).To(Equal(strings.Repeat("a", log_streamer.MAX_MESSAGE_SIZE-3)))
					ms, _, _ = fakeClient.SendAppLogArgsForCall(1)
					Expect(ms).To(Equal("\U0001F428"))
				})

				Context("with an invalid utf8 character in the message", func() {
					var utfChar string

					BeforeEach(func() {
						message = strings.Repeat("9", log_streamer.MAX_MESSAGE_SIZE-4)
						utfChar = "\U0001F428"
					})

					It("emits both messages correctly", func() {
						fmt.Fprint(streamer.Stdout(), message+utfChar[0:2])
						fmt.Fprintln(streamer.Stdout(), utfChar)

						Expect(fakeClient.SendAppLogCallCount()).To(Equal(2))

						ms, _, _ := fakeClient.SendAppLogArgsForCall(0)
						Expect(ms).To(Equal(message + utfChar[0:2]))

						ms, _, _ = fakeClient.SendAppLogArgsForCall(1)
						Expect(ms).To(Equal(utfChar))
					})
				})

				Context("when the entire message is invalid utf8 characters", func() {
					var utfChar string

					BeforeEach(func() {
						utfChar = "\U0001F428"
						message = strings.Repeat(utfChar[0:2], log_streamer.MAX_MESSAGE_SIZE/2)
						Expect(len(message)).To(Equal(log_streamer.MAX_MESSAGE_SIZE))
					})

					It("drops the last 3 bytes", func() {
						fmt.Fprint(streamer.Stdout(), message)

						Expect(fakeClient.SendAppLogCallCount()).To(Equal(1))

						ms, _, _ := fakeClient.SendAppLogArgsForCall(0)
						Expect(ms).To(Equal(message[0 : len(message)-3]))
					})
				})
			})

			Context("while concatenating, if the message exceeds the emittable length", func() {
				BeforeEach(func() {
					message = strings.Repeat("7", log_streamer.MAX_MESSAGE_SIZE-2)
					fmt.Fprint(streamer.Stdout(), message)
					fmt.Fprint(streamer.Stdout(), "778888\n")
				})

				It("should break the message up and send multiple messages", func() {
					Expect(fakeClient.SendAppLogCallCount()).To(Equal(2))

					ms, _, _ := fakeClient.SendAppLogArgsForCall(0)
					Expect(ms).To(Equal(strings.Repeat("7", log_streamer.MAX_MESSAGE_SIZE)))
					ms, _, _ = fakeClient.SendAppLogArgsForCall(1)
					Expect(ms).To(Equal("8888"))
				})
			})
		})
	})

	Context("when told to emit stderr", func() {
		It("should handle short messages", func() {
			fmt.Fprintf(streamer.Stderr(), "this is a log\nand this is another\nand this one isn't done yet...")
			Expect(fakeClient.SendAppErrorLogCallCount()).To(Equal(2))

			msg, sn, _ := fakeClient.SendAppErrorLogArgsForCall(0)
			Expect(msg).To(Equal("this is a log"))
			Expect(sn).To(Equal(sourceName))

			msg, sn, _ = fakeClient.SendAppErrorLogArgsForCall(1)
			Expect(msg).To(Equal("and this is another"))
			Expect(sn).To(Equal(sourceName))
		})

		It("should handle long messages", func() {
			fmt.Fprintln(streamer.Stderr(), strings.Repeat("e", log_streamer.MAX_MESSAGE_SIZE+1))
			Expect(fakeClient.SendAppErrorLogCallCount()).To(Equal(2))

			msg, _, _ := fakeClient.SendAppErrorLogArgsForCall(0)
			Expect(msg).To(Equal(strings.Repeat("e", log_streamer.MAX_MESSAGE_SIZE)))

			msg, _, _ = fakeClient.SendAppErrorLogArgsForCall(1)
			Expect(msg).To(Equal("e"))
		})
	})

	Context("when told to flush", func() {
		It("should send whatever log is left in its buffer", func() {
			fmt.Fprintf(streamer.Stdout(), "this is a stdout")
			fmt.Fprintf(streamer.Stderr(), "this is a stderr")

			Expect(fakeClient.SendAppLogCallCount()).To(Equal(0))
			Expect(fakeClient.SendAppErrorLogCallCount()).To(Equal(0))

			streamer.Flush()

			Expect(fakeClient.SendAppLogCallCount()).To(Equal(1))
			Expect(fakeClient.SendAppErrorLogCallCount()).To(Equal(1))
		})
	})

	Context("when there is no app guid", func() {
		It("does nothing when told to emit or flush", func() {
			logConfig = models.LogConfig{Guid: "", SourceName: sourceName, Index: index, Tags: tags}
			streamer = log_streamer.New(logConfig, fakeClient, maxLogLinesPerSecond, maxLogBytesPerSecond, metricReportInterval)

			streamer.Stdout().Write([]byte("hi"))
			streamer.Stderr().Write([]byte("hi"))
			streamer.Flush()

			Expect(fakeClient.SendAppLogCallCount()).To(Equal(0))
		})
	})

	Context("when there is no log source", func() {
		It("defaults to LOG", func() {
			logConfig = models.LogConfig{Guid: guid, SourceName: "", Index: index, Tags: tags}
			streamer = log_streamer.New(logConfig, fakeClient, maxLogLinesPerSecond, maxLogBytesPerSecond, metricReportInterval)

			streamer.Stdout().Write([]byte("hi"))
			streamer.Flush()

			Expect(fakeClient.SendAppLogCallCount()).To(Equal(1))
			_, sn, _ := fakeClient.SendAppLogArgsForCall(0)
			Expect(sn).To(Equal(log_streamer.DefaultLogSource))
		})
	})

	Context("when there is no source index", func() {
		It("defaults to 0", func() {
			logConfig = models.LogConfig{Guid: guid, SourceName: sourceName, Index: -1, Tags: tags}
			streamer = log_streamer.New(logConfig, fakeClient, maxLogLinesPerSecond, maxLogBytesPerSecond, metricReportInterval)

			streamer.Stdout().Write([]byte("hi"))
			streamer.Flush()

			Expect(fakeClient.SendAppLogCallCount()).To(Equal(1))
			_, _, tags := fakeClient.SendAppLogArgsForCall(0)
			Expect(tags["instance_id"]).To(Equal("-1"))
		})
	})

	Context("with multiple goroutines emitting simultaneously", func() {
		var waitGroup *sync.WaitGroup

		BeforeEach(func() {
			waitGroup = new(sync.WaitGroup)

			for i := 0; i < 2; i++ {
				waitGroup.Add(1)
				go func() {
					defer waitGroup.Done()
					fmt.Fprintln(streamer.Stdout(), "this is a log")
				}()
			}
		})

		AfterEach(func() {
			done := make(chan interface{})
			go func() {
				defer close(done)
				waitGroup.Wait()
			}()
			Eventually(done).Should(BeClosed())
		})

		It("does not trigger data races", func() {
			Eventually(fakeClient.SendAppLogCallCount).Should(Equal(2))
		})
	})

	Describe("UpdateTags", func() {
		Context("when updated with new tags", func() {
			var updatedTags map[string]string
			BeforeEach(func() {
				updatedTags = map[string]string{"new-tag": "some-value"}
				streamer.UpdateTags(updatedTags)
			})

			It("emits stdout messages with the updated tags", func() {
				_, err := fmt.Fprintln(streamer.Stdout(), "this is a log")
				Expect(err).ToNot(HaveOccurred())
				_, err = fmt.Fprintln(streamer.Stdout(), "this is another log")
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeClient.SendAppLogCallCount()).To(Equal(2))

				_, _, tags := fakeClient.SendAppLogArgsForCall(0)
				Expect(tags).To(Equal(updatedTags))

				_, _, tags = fakeClient.SendAppLogArgsForCall(1)
				Expect(tags).To(Equal(updatedTags))
			})

			It("emits stderr messages with the updated tags", func() {
				_, err := fmt.Fprintln(streamer.Stderr(), "this is a log")
				Expect(err).ToNot(HaveOccurred())
				_, err = fmt.Fprintln(streamer.Stderr(), "this is another log")
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeClient.SendAppErrorLogCallCount()).To(Equal(2))

				_, _, tags := fakeClient.SendAppErrorLogArgsForCall(0)
				Expect(tags).To(Equal(updatedTags))

				_, _, tags = fakeClient.SendAppErrorLogArgsForCall(1)
				Expect(tags).To(Equal(updatedTags))
			})
		})

		Context("with multiple goroutines emitting simultaneously", func() {
			var waitGroup *sync.WaitGroup
			It("does not trigger data races", func() {
				waitGroup = new(sync.WaitGroup)

				waitGroup.Add(1)
				go func() {
					defer waitGroup.Done()
					streamer.UpdateTags(map[string]string{"some-tag": "some-value"})
				}()

				for i := 0; i < 2; i++ {
					waitGroup.Add(1)
					go func() {
						defer waitGroup.Done()
						fmt.Fprintln(streamer.Stdout(), "this is a log")
					}()
				}
				waitGroup.Wait()

				Eventually(fakeClient.SendAppLogCallCount).Should(Equal(2))
			})
		})

	})

	Describe("Stop", func() {
		Context("stopping the log streamer", func() {
			BeforeEach(func() {
				streamer.Stop()
			})

			It("writes to stdout and stderr should not fail", func() {
				_, stdOutErr := fmt.Fprintln(streamer.Stdout(), "this is a log")
				Expect(stdOutErr).NotTo(HaveOccurred())
				_, stdErrErr := fmt.Fprintln(streamer.Stderr(), "this is another log")
				Expect(stdErrErr).NotTo(HaveOccurred())

			})

			Context("when a 'child' log streamer is present", func() {
				var childStreamer log_streamer.LogStreamer

				BeforeEach(func() {
					childStreamer = streamer.WithSource("CHILD")
				})

				It("writes to the child's stdout and stderr should not fail", func() {
					_, stdOutErr := fmt.Fprintln(childStreamer.Stdout(), "this is a log")
					Expect(stdOutErr).NotTo(HaveOccurred())
					_, stdErrErr := fmt.Fprintln(childStreamer.Stderr(), "this is another log")
					Expect(stdErrErr).NotTo(HaveOccurred())
				})
			})
		})

		Context("when a 'child' log streamer is stopped", func() {
			var childStreamer log_streamer.LogStreamer

			BeforeEach(func() {
				childStreamer = streamer.WithSource("CHILD")
				childStreamer.Stop()
			})

			It("writes to the child's stdout and stderr should not fail", func() {
				_, stdOutErr := fmt.Fprintln(childStreamer.Stdout(), "this is a log")
				Expect(stdOutErr).NotTo(HaveOccurred())
				_, stdErrErr := fmt.Fprintln(childStreamer.Stderr(), "this is another log")
				Expect(stdErrErr).NotTo(HaveOccurred())
			})

			It("writes to the parent's stdout and stderr should continue to succeed", func() {
				_, stdOutErr := fmt.Fprintln(streamer.Stdout(), "this is a log")
				Expect(stdOutErr).NotTo(HaveOccurred())
				_, stdErrErr := fmt.Fprintln(streamer.Stderr(), "this is another log")
				Expect(stdErrErr).NotTo(HaveOccurred())
			})
		})
	})
})
