package rundmc

//counterfeiter:generate . MountPointChecker
type MountPointChecker func(path string) (bool, error)

//counterfeiter:generate . MountOptionsGetter
type MountOptionsGetter func(path string) ([]string, error)
