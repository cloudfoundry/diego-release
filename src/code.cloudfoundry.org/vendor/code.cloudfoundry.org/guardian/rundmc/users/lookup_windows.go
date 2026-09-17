package users

import "fmt"

const (
	DefaultHome string = `C:\\Users\\ContainerAdministrator`
)

func LookupUser(rootFsPath, userName string) (*ExecUser, error) {
	user := &ExecUser{Uid: DefaultUID, Gid: DefaultGID, Home: DefaultHome}
	if userName != "" {
		user.Home = fmt.Sprintf("C:\\Users\\%s", userName)
	}
	return user, nil
}
