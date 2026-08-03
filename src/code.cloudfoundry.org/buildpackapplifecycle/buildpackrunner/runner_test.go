package buildpackrunner_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"code.cloudfoundry.org/buildpackapplifecycle"
	"code.cloudfoundry.org/buildpackapplifecycle/buildpackrunner"
	"code.cloudfoundry.org/buildpackapplifecycle/test_helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Runner", func() {
	Describe("GoLikeLightning", func() {
		Describe("writing the start command to results.json and staging_info.yml", func() {

			var runner *buildpackrunner.Runner
			var buildpacks = []string{"haskell-buildpack", "bash-buildpack"}
			var builderConfig buildpackapplifecycle.LifecycleBuilderConfig

			var defaultStartCommandFromFixtures = "This is the start command for the 'web' default process type in testdata/fake_{unix,windows}_bp/bin/release{,.bat}"

			BeforeEach(func() {
				builderConfig = makeBuilderConfig(buildpacks, fakeBuildpackDir())
				runner = buildpackrunner.New(&builderConfig)
				Expect(runner.Setup()).To(Succeed())
			})

			When("There is NO procfile and NO launch.yml file", func() {
				It("should use the default start command", func() {
					resultsJSON, stagingInfo, err := runner.GoLikeLightning()

					Expect(err).NotTo(HaveOccurred())
					Expect(stagingInfo).To(ContainSubstring("staging_info.yml"))
					Expect(stagingInfo).To(BeAnExistingFile())

					stagingInfoContents, err := os.ReadFile(stagingInfo)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(stagingInfoContents)).To(ContainSubstring(fmt.Sprintf(`{"detected_buildpack":"","start_command":"%s"}`, defaultStartCommandFromFixtures)))

					resultsJSONContents, err := os.ReadFile(resultsJSON)
					Expect(err).ToNot(HaveOccurred())

					actualStagingResult := buildpackapplifecycle.StagingResult{}
					Expect(json.Unmarshal(resultsJSONContents, &actualStagingResult)).To(Succeed())

					Expect(actualStagingResult.ProcessTypes).To(Equal(buildpackapplifecycle.ProcessTypes{"web": defaultStartCommandFromFixtures}))
					Expect(actualStagingResult.ProcessList).To(Equal([]buildpackapplifecycle.Process{{Type: "web", Command: defaultStartCommandFromFixtures}}))
				})
			})

			When("A launch.yml is present and there is NO procfile", func() {
				var launchContent = []string{`
processes:
- type: "web"
  command: "do something forever"
- type: "worker"
  command: "do something and then quit"
- type: "newrelic"
  command: "run new relic"
  platforms:
    cloudfoundry:
      sidecar_for: [ "web" , "worker" ] `, `
processes:
- type: "web"
  command: "do something else forever"
- type: "oldrelic"
  command: "run new relic"
  limits:
    memory: 10
  platforms:
    cloudfoundry:
      sidecar_for: [ "web" ] `}

				BeforeEach(func() {
					Expect(os.MkdirAll(runner.GetDepsDir(), os.ModePerm)).To(Succeed())

					for index := range buildpacks {
						depsIdxPath := filepath.Join(runner.GetDepsDir(), strconv.Itoa(index))
						Expect(os.MkdirAll(depsIdxPath, os.ModePerm)).To(Succeed())
						launchPath := filepath.Join(depsIdxPath, "launch.yml")
						Expect(os.WriteFile(launchPath, []byte(launchContent[index]), os.ModePerm)).To(Succeed())
					}
				})

				AfterEach(func() {
					os.RemoveAll(runner.GetDepsDir())
				})

				It("Should use the start command from launch.yml", func() {
					resultsJSON, stagingInfo, err := runner.GoLikeLightning()

					Expect(err).NotTo(HaveOccurred())
					Expect(stagingInfo).To(ContainSubstring("staging_info.yml"))
					Expect(stagingInfo).To(BeAnExistingFile())

					stagingInfoContents, err := os.ReadFile(stagingInfo)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(stagingInfoContents)).To(ContainSubstring(`{"detected_buildpack":"","start_command":"do something else forever"}`))

					resultsJSONContents, err := os.ReadFile(resultsJSON)
					Expect(err).ToNot(HaveOccurred())

					actualStagingResult := buildpackapplifecycle.StagingResult{}
					Expect(json.Unmarshal(resultsJSONContents, &actualStagingResult)).To(Succeed())

					Expect(actualStagingResult.ProcessTypes).To(Equal(buildpackapplifecycle.ProcessTypes{
						"web":    "do something else forever",
						"worker": "do something and then quit",
					}))

					Expect(actualStagingResult.ProcessList).To(Equal([]buildpackapplifecycle.Process{
						{Type: "web", Command: "do something else forever"},
						{Type: "worker", Command: "do something and then quit"},
					}))

					Expect(actualStagingResult.Sidecars).To(Equal([]buildpackapplifecycle.Sidecar{
						{Name: "newrelic", ProcessTypes: []string{"web", "worker"}, Command: "run new relic"},
						{Name: "oldrelic", ProcessTypes: []string{"web"}, Command: "run new relic", Memory: 10},
					}))

				})
			})

			When("A procfile is present and there is NO launch.yml", func() {
				var procFilePath string
				BeforeEach(func() {
					procFilePath = filepath.Join(builderConfig.BuildDir(), "Procfile")
					Expect(os.WriteFile(procFilePath, []byte("web: gunicorn server:app"), os.ModePerm)).To(Succeed())
				})

				AfterEach(func() {
					os.Remove(procFilePath)
				})

				It("Should always use the start command from the procfile", func() {
					resultsJSON, stagingInfo, err := runner.GoLikeLightning()

					Expect(err).NotTo(HaveOccurred())
					Expect(stagingInfo).To(ContainSubstring("staging_info.yml"))
					Expect(stagingInfo).To(BeAnExistingFile())

					contents, err := os.ReadFile(stagingInfo)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(contents)).To(ContainSubstring(`{"detected_buildpack":"","start_command":"gunicorn server:app"}`))

					resultsJSONContents, err := os.ReadFile(resultsJSON)
					Expect(err).ToNot(HaveOccurred())

					actualStagingResult := buildpackapplifecycle.StagingResult{}
					Expect(json.Unmarshal(resultsJSONContents, &actualStagingResult)).To(Succeed())

					Expect(actualStagingResult.ProcessTypes).To(Equal(buildpackapplifecycle.ProcessTypes{"web": "gunicorn server:app"}))
					Expect(actualStagingResult.ProcessList).To(Equal([]buildpackapplifecycle.Process{{Type: "web", Command: "gunicorn server:app"}}))
				})
			})

			When("there is NO procfile present and there is launch.yml provided by supply buildpacks", func() {
				var launchContents = `
processes:
- type: "web"
  command: "do something forever"
- type: "worker"
  command: "do something and then quit"
- type: "lightning"
  command: "go forth"
- type: "newrelic"
  command: "run new relic"
  platforms:
    cloudfoundry:
      sidecar_for: [ "web" ]`

				BeforeEach(func() {
					depsIdxPath := filepath.Join(runner.GetDepsDir(), strconv.Itoa(0))
					Expect(os.MkdirAll(depsIdxPath, os.ModePerm)).To(Succeed())
					launchPath := filepath.Join(depsIdxPath, "launch.yml")
					Expect(os.WriteFile(launchPath, []byte(launchContents), os.ModePerm)).To(Succeed())
				})

				It("Should always use the start command from the bin/release", func() {
					resultsJSON, stagingInfo, err := runner.GoLikeLightning()

					Expect(err).NotTo(HaveOccurred())
					Expect(stagingInfo).To(ContainSubstring("staging_info.yml"))
					Expect(stagingInfo).To(BeAnExistingFile())

					stagingInfoContents, err := os.ReadFile(stagingInfo)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(stagingInfoContents)).To(ContainSubstring(fmt.Sprintf(`{"detected_buildpack":"","start_command":"%s"}`, defaultStartCommandFromFixtures)))

					resultsJSONContents, err := os.ReadFile(resultsJSON)
					Expect(err).ToNot(HaveOccurred())

					actualStagingResult := buildpackapplifecycle.StagingResult{}
					Expect(json.Unmarshal(resultsJSONContents, &actualStagingResult)).To(Succeed())

					Expect(actualStagingResult.ProcessTypes).To(Equal(buildpackapplifecycle.ProcessTypes{
						"lightning": "go forth",
						"web":       defaultStartCommandFromFixtures,
						"worker":    "do something and then quit",
					}))

					Expect(actualStagingResult.ProcessList).To(Equal([]buildpackapplifecycle.Process{
						{Type: "web", Command: defaultStartCommandFromFixtures},
						{Type: "worker", Command: "do something and then quit"},
						{Type: "lightning", Command: "go forth"},
					}))

					Expect(actualStagingResult.Sidecars).To(Equal([]buildpackapplifecycle.Sidecar{
						{Name: "newrelic", ProcessTypes: []string{"web"}, Command: "run new relic"},
					}))
				})
			})

			When("A procfile is present and there is launch.yml provided by all buildpacks", func() {
				var procFilePath string
				var launchContent = []string{`
processes:
- type: "web"
  command: "do something forever"
- type: "worker"
  command: "do something and then quit"
- type: "lightning"
  command: "go forth"
- type: "newrelic"
  command: "run new relic"
  platforms:
    cloudfoundry:
      sidecar_for: [ "web" ] `, `
processes:
- type: "worker"
  command: "do something else forever"
- type: "oldrelic"
  command: "run new relic"
  limits:
    memory: 10
  platforms:
    cloudfoundry:
      sidecar_for: [ "worker" ] `}

				BeforeEach(func() {
					procFilePath := filepath.Join(builderConfig.BuildDir(), "Procfile")
					Expect(os.WriteFile(procFilePath, []byte("web: gunicorn server:app"), os.ModePerm)).To(Succeed())

					for index := range buildpacks {
						depsIdxPath := filepath.Join(runner.GetDepsDir(), strconv.Itoa(index))
						Expect(os.MkdirAll(depsIdxPath, os.ModePerm)).To(Succeed())
						launchPath := filepath.Join(depsIdxPath, "launch.yml")
						Expect(os.WriteFile(launchPath, []byte(launchContent[index]), os.ModePerm)).To(Succeed())
					}
				})

				AfterEach(func() {
					os.Remove(procFilePath)
				})

				It("Should always use the start command from the procfile", func() {
					resultsJSON, stagingInfo, err := runner.GoLikeLightning()

					Expect(err).NotTo(HaveOccurred())
					Expect(stagingInfo).To(ContainSubstring("staging_info.yml"))
					Expect(stagingInfo).To(BeAnExistingFile())

					stagingInfoContents, err := os.ReadFile(stagingInfo)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(stagingInfoContents)).To(ContainSubstring(`{"detected_buildpack":"","start_command":"gunicorn server:app"}`))

					resultsJSONContents, err := os.ReadFile(resultsJSON)
					Expect(err).ToNot(HaveOccurred())

					actualStagingResult := buildpackapplifecycle.StagingResult{}
					Expect(json.Unmarshal(resultsJSONContents, &actualStagingResult)).To(Succeed())

					Expect(actualStagingResult.ProcessTypes).To(Equal(buildpackapplifecycle.ProcessTypes{
						"lightning": "go forth",
						"web":       "gunicorn server:app",
						"worker":    "do something else forever",
					}))

					Expect(actualStagingResult.ProcessList).To(Equal([]buildpackapplifecycle.Process{
						{Type: "web", Command: "gunicorn server:app"},
						{Type: "worker", Command: "do something else forever"},
						{Type: "lightning", Command: "go forth"},
					}))

					Expect(actualStagingResult.Sidecars).To(Equal([]buildpackapplifecycle.Sidecar{
						{Name: "newrelic", ProcessTypes: []string{"web"}, Command: "run new relic"},
						{Name: "oldrelic", ProcessTypes: []string{"worker"}, Command: "run new relic", Memory: 10},
					}))
				})
			})
		})
	})

	Describe("GoLikeLightning failure scenarios", func() {
		var runner *buildpackrunner.Runner
		var builderConfig buildpackapplifecycle.LifecycleBuilderConfig

		When("staging fails but buildpack has written metadata", func() {
			BeforeEach(func() {
				buildpacks := []string{"failing-buildpack"}
				builderConfig = makeBuilderConfig(buildpacks, fakeFailingBuildpackDir())
				runner = buildpackrunner.New(&builderConfig)
				Expect(runner.Setup()).To(Succeed())
			})

			It("should write result.json with buildpack metadata on failure", func() {
				_, _, err := runner.GoLikeLightning()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Failed to compile droplet"))

				resultsJSONPath := builderConfig.OutputMetadata()
				Expect(resultsJSONPath).To(BeAnExistingFile())

				resultsJSONContents, err := os.ReadFile(resultsJSONPath)
				Expect(err).ToNot(HaveOccurred())

				actualStagingResult := buildpackapplifecycle.StagingResult{}
				Expect(json.Unmarshal(resultsJSONContents, &actualStagingResult)).To(Succeed())

				Expect(actualStagingResult.LifecycleType).To(Equal("buildpack"))
				Expect(actualStagingResult.LifecycleMetadata.Buildpacks).To(HaveLen(1))
				Expect(actualStagingResult.LifecycleMetadata.Buildpacks[0].Key).To(Equal("failing-buildpack"))
				Expect(actualStagingResult.LifecycleMetadata.Buildpacks[0].Name).To(Equal("failing-buildpack"))
				Expect(actualStagingResult.LifecycleMetadata.Buildpacks[0].Version).To(Equal("1.2.3"))
			})
		})

		When("staging fails with multiple buildpacks", func() {
			BeforeEach(func() {
				buildpacks := []string{"first-failing-buildpack", "second-failing-buildpack"}
				builderConfig = makeBuilderConfig(buildpacks, fakeFailingBuildpackDir())
				runner = buildpackrunner.New(&builderConfig)
				Expect(runner.Setup()).To(Succeed())
			})

			It("should write result.json with metadata for all buildpacks on failure", func() {
				_, _, err := runner.GoLikeLightning()
				Expect(err).To(HaveOccurred())

				resultsJSONPath := builderConfig.OutputMetadata()
				Expect(resultsJSONPath).To(BeAnExistingFile())

				resultsJSONContents, err := os.ReadFile(resultsJSONPath)
				Expect(err).ToNot(HaveOccurred())

				actualStagingResult := buildpackapplifecycle.StagingResult{}
				Expect(json.Unmarshal(resultsJSONContents, &actualStagingResult)).To(Succeed())

				Expect(actualStagingResult.LifecycleType).To(Equal("buildpack"))
				Expect(actualStagingResult.LifecycleMetadata.Buildpacks).To(HaveLen(2))

				Expect(actualStagingResult.LifecycleMetadata.Buildpacks[0].Key).To(Equal("first-failing-buildpack"))
				Expect(actualStagingResult.LifecycleMetadata.Buildpacks[0].Name).To(Equal("failing-buildpack"))
				Expect(actualStagingResult.LifecycleMetadata.Buildpacks[0].Version).To(Equal("1.2.3"))

				Expect(actualStagingResult.LifecycleMetadata.Buildpacks[1].Key).To(Equal("second-failing-buildpack"))
				Expect(actualStagingResult.LifecycleMetadata.Buildpacks[1].Name).To(Equal("failing-buildpack"))
				Expect(actualStagingResult.LifecycleMetadata.Buildpacks[1].Version).To(Equal("1.2.3"))
			})
		})
	})
})

func makeBuilderConfig(buildpacks []string, testdataDir string) buildpackapplifecycle.LifecycleBuilderConfig {
	skipDetect := true
	builderConfig := buildpackapplifecycle.NewLifecycleBuilderConfig(buildpacks, skipDetect, false)
	outputMetadataPath, err := os.MkdirTemp(os.TempDir(), "results")
	Expect(err).ToNot(HaveOccurred())
	Expect(builderConfig.Set("outputMetadata", filepath.Join(outputMetadataPath, "results.json"))).To(Succeed())

	buildDirPath, err := os.MkdirTemp(os.TempDir(), "app")
	Expect(err).ToNot(HaveOccurred())
	Expect(builderConfig.Set("buildDir", buildDirPath)).To(Succeed())

	buildpacksDirPath, err := os.MkdirTemp(os.TempDir(), "buildpack")
	Expect(err).ToNot(HaveOccurred())
	Expect(builderConfig.Set("buildpacksDir", buildpacksDirPath)).To(Succeed())

	for _, bp := range buildpacks {
		bpPath := builderConfig.BuildpackPath(bp)
		Expect(genFakeBuildpack(bpPath, testdataDir)).To(Succeed())
	}

	err = os.MkdirAll(builderConfig.BuildDir(), os.ModePerm)
	Expect(err).ToNot(HaveOccurred())

	if runtime.GOOS == "windows" {
		copyDst := filepath.Join(filepath.Dir(builderConfig.Path()), "tar.exe")
		test_helpers.CopyFile(tmpTarPath, copyDst)
	}

	return builderConfig
}

func genFakeBuildpack(bpRoot string, testdataDir string) error {
	if err := os.MkdirAll(filepath.Join(bpRoot, "bin"), os.ModePerm); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		CopyDirectory(filepath.Join("testdata", testdataDir, "bin", "*"), filepath.Join(bpRoot, "bin"))
	} else {
		CopyDirectory(filepath.Join("testdata", testdataDir, "bin"), filepath.Join(bpRoot))
	}
	return nil
}

func CopyDirectory(src string, dst string) {
	var command *exec.Cmd
	if runtime.GOOS == "windows" {
		command = exec.Command("powershell", "-Command", "Copy-Item", "-Recurse", "-Force", src, dst)
	} else {
		command = exec.Command("cp", "-a", "-R", src, dst)
	}

	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	session.Wait()
	Expect(session).Should(gexec.Exit())
}

func fakeBuildpackDir() string {
	if runtime.GOOS == "windows" {
		return "fake_windows_bp"
	}
	return "fake_unix_bp"
}

func fakeFailingBuildpackDir() string {
	if runtime.GOOS == "windows" {
		return "fake_windows_bp_failing"
	}
	return "fake_unix_bp_failing"
}
