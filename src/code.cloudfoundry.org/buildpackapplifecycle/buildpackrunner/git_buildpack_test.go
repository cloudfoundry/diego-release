package buildpackrunner_test

import (
	"os"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/buildpackapplifecycle/buildpackrunner"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GitBuildpack", func() {

	Describe("Clone", func() {
		var cloneTarget string
		BeforeEach(func() {
			var err error
			cloneTarget, err = os.MkdirTemp(tmpDir, "clone")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			os.RemoveAll(cloneTarget)
		})

		Context("With a Git transport that doesn't support `--depth`", func() {
			It("clones a URL", func() {
				err := buildpackrunner.GitClone(gitUrl, cloneTarget)
				Expect(err).NotTo(HaveOccurred())
				Expect(currentBranch(cloneTarget)).To(Equal("master"))
			})

			It("clones a URL with a branch", func() {
				branchUrl := gitUrl
				branchUrl.Fragment = "a_branch"
				err := buildpackrunner.GitClone(branchUrl, cloneTarget)
				Expect(err).NotTo(HaveOccurred())
				Expect(currentBranch(cloneTarget)).To(Equal("a_branch"))
			})

			It("clones a URL with a lightweight tag", func() {
				branchUrl := gitUrl
				branchUrl.Fragment = "a_lightweight_tag"
				err := buildpackrunner.GitClone(branchUrl, cloneTarget)
				Expect(err).NotTo(HaveOccurred())
				Expect(currentBranch(cloneTarget)).To(Equal("a_lightweight_tag"))
			})

			Context("when git repo has submodules", func() {
				It("updates the submodules for the branch", func() {
					branchUrl := gitUrl
					branchUrl.Fragment = "a_branch"
					err := buildpackrunner.GitClone(branchUrl, cloneTarget)
					Expect(err).NotTo(HaveOccurred())

					fileContents, _ := os.ReadFile(cloneTarget + "/sub/README")
					Expect(string(fileContents)).To(Equal("2nd commit"))
				})
			})

			Context("with bogus git URLs", func() {
				It("returns an error", func() {
					By("passing an invalid path", func() {
						badUrl := gitUrl
						badUrl.Path = "/a/bad/path"
						err := buildpackrunner.GitClone(badUrl, cloneTarget)
						Expect(err).To(HaveOccurred())
					})

					By("passing a bad tag/branch", func() {
						badUrl := gitUrl
						badUrl.Fragment = "notfound"
						err := buildpackrunner.GitClone(badUrl, cloneTarget)
						Expect(err).To(HaveOccurred())
					})
				})
			})
		})

		Context("With a Git transport that supports `--depth`", func() {
			It("clones a URL", func() {
				err := buildpackrunner.GitClone(fileGitUrl, cloneTarget)
				Expect(err).NotTo(HaveOccurred())
				Expect(currentBranch(cloneTarget)).To(Equal("master"))
			})

			It("clones a URL with a branch", func() {
				branchUrl := fileGitUrl
				branchUrl.Fragment = "a_branch"
				err := buildpackrunner.GitClone(branchUrl, cloneTarget)
				Expect(err).NotTo(HaveOccurred())
				Expect(currentBranch(cloneTarget)).To(Equal("a_branch"))
			})

			It("clones a URL with a lightweight tag", func() {
				branchUrl := fileGitUrl
				branchUrl.Fragment = "a_lightweight_tag"
				err := buildpackrunner.GitClone(branchUrl, cloneTarget)
				Expect(err).NotTo(HaveOccurred())
				Expect(currentBranch(cloneTarget)).To(Equal("a_lightweight_tag"))
			})

			It("does a shallow clone of the repo", func() {
				gitPath, err := exec.LookPath("git")
				Expect(err).NotTo(HaveOccurred())
				version, err := exec.Command(gitPath, "version").CombinedOutput()
				Expect(err).NotTo(HaveOccurred())
				if string(version) == "git version 2.9.0\n" {
					Skip("shallow clone not support with submodules for git 2.9.0")
				}

				buildpackrunner.GitClone(fileGitUrl, cloneTarget)

				cmd := exec.Command("git", "rev-list", "HEAD", "--count")
				cmd.Dir = cloneTarget
				bytes, err := cmd.Output()
				output := strings.TrimSpace(string(bytes))

				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("1"))
			})
		})
	})
})

func currentBranch(gitDir string) string {
	cmd := exec.Command("git", "symbolic-ref", "--short", "-q", "HEAD")
	cmd.Dir = gitDir
	bytes, err := cmd.Output()
	if err != nil {
		// try the tag
		cmd := exec.Command("git", "name-rev", "--name-only", "--tags", "HEAD")
		cmd.Dir = gitDir
		bytes, err = cmd.Output()
	}
	Expect(err).NotTo(HaveOccurred())
	return strings.TrimSpace(string(bytes))
}
