package cfattestor

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// containerHandle reads <procRoot>/<pid>/cgroup and returns the Garden handle.
func containerHandle(procRoot string, pid int) (string, error) {
	content, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "cgroup"))
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Split(line, ":")
		path := fields[len(fields)-1]
		if handle, ok := handleFromPath(path); ok {
			return handle, nil
		}
	}

	return "", fmt.Errorf("no garden cgroup for pid %d", pid)
}

// handleFromPath extracts the path segment immediately following "garden".
// v1 line: "<id>:<controller>:/garden/<handle>"
// v2 line: "0::/garden/<handle>/init" (or deeper)
func handleFromPath(p string) (string, bool) {
	segments := strings.Split(p, "/")
	for i, seg := range segments {
		if seg == "garden" && i+1 < len(segments) && segments[i+1] != "" {
			return segments[i+1], true
		}
	}
	return "", false
}
