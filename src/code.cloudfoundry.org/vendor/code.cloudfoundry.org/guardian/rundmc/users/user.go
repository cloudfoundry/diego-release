package users

const (
	DefaultUID int = 0
	DefaultGID int = 0
)

//counterfeiter:generate . UserLookupper
type UserLookupper interface {
	Lookup(rootFsPath string, user string) (*ExecUser, error)
}

type LookupFunc func(rootfsPath, user string) (*ExecUser, error)

func (fn LookupFunc) Lookup(rootfsPath, user string) (*ExecUser, error) {
	return fn(rootfsPath, user)
}

type ExecUser struct {
	Uid   int
	Gid   int
	Sgids []int
	Home  string
}
