package buildpackrunner

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
)

func GitClone(repo url.URL, destination string) error {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return err
	}

	branch := repo.Fragment
	repo.Fragment = ""
	gitUrl := repo.String()

	err = performGitClone(gitPath,
		[]string{
			"--depth",
			"1",
			"--recursive",
			gitUrl,
			destination,
		}, branch)

	if err != nil {
		err = os.RemoveAll(destination)
		if err != nil {
			return fmt.Errorf("Failed to remove %s prior to cloning on top: %s", destination, err)
		}

		err = performGitClone(gitPath,
			[]string{
				"--recursive",
				gitUrl,
				destination,
			}, branch)

		if err != nil {
			return fmt.Errorf("Failed to clone git repository at %s: %w", gitUrl, err)
		}
	}

	return nil
}

func performGitClone(gitPath string, args []string, branch string) error {
	args = append([]string{"clone"}, args...)

	if branch != "" {
		args = append(args, "-b", branch)
	}
	cmd := exec.Command(gitPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("git clone failed:\n%s\n%w\n", output, err)
	}

	return err
}
