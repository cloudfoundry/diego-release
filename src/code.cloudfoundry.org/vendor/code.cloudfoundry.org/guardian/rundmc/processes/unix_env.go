package processes

import (
	"regexp"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

const DefaultRootPath = "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
const DefaultPath = "PATH=/usr/local/bin:/usr/bin:/bin"

func envWithDefaultPath(defaultPath string, env []string) []string {
	pathRegexp := regexp.MustCompile("^PATH=.*$")

	for _, envVar := range env {
		if pathRegexp.MatchString(envVar) {
			return env
		}
	}

	return append(env, defaultPath)
}

func envWithUser(env []string, user string) []string {
	userRegexp := regexp.MustCompile("^USER=.*$")
	for _, envVar := range env {
		if userRegexp.MatchString(envVar) {
			return env
		}
	}

	if user != "" {
		return append(env, "USER="+user)
	}

	return append(env, "USER=root")
}

func UnixEnvFor(bndl goci.Bndl, spec garden.ProcessSpec, containerUID int) []string {
	requestedEnv := append(bndl.Spec.Process.Env, spec.Env...)

	defaultPath := DefaultPath
	if containerUID == 0 {
		defaultPath = DefaultRootPath
	}

	return envWithUser(envWithDefaultPath(defaultPath, requestedEnv), spec.User)
}
