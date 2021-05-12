package rundmc

//go:generate counterfeiter . MountPointChecker
type MountPointChecker func(path string) (bool, error)

//go:generate counterfeiter . MountOptionsGetter
type MountOptionsGetter func(path string) ([]string, error)
