// +build !linux

package runner

func MustMountTmpfs(destination string) {
	panic("not supported")
}

func MustUnmountTmpfs(destination string) {
	panic("not supported")
}

func Unmount(destination string) {
	panic("not supported")
}
