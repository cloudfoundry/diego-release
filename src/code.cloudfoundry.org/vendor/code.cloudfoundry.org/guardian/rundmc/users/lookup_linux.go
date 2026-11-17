package users

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	pathrs "github.com/cyphar/filepath-securejoin/pathrs-lite"
	"github.com/moby/sys/user"
	"golang.org/x/sys/unix"
)

const (
	DefaultHome string = "/"

	// Same cap and semantics as github.com/moby/sys/user maxUserFileBytes.
	maxUserFileBytes = 10 << 20
)

var errNotRegularFile = errors.New("not a regular file")

type limitedFile struct {
	*os.File
	lr   *io.LimitedReader
	name string
}

func (l *limitedFile) Read(p []byte) (int, error) {
	n, err := l.lr.Read(p)
	if l.lr.N == 0 {
		return n, &os.PathError{Op: "read", Path: l.name, Err: fmt.Errorf("file exceeds %d bytes", maxUserFileBytes)}
	}
	return n, err
}

// openUserFileInRoot resolves path relative to root (symlinks are resolved as if root were /),
// requires a regular file via fstat on an O_PATH handle, then reopens it for reading.
func openUserFileInRoot(root, path string) (*limitedFile, error) {
	h, err := pathrs.OpenInRoot(root, path)
	if err != nil {
		return nil, err
	}
	defer h.Close()

	fi, err := h.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, errNotRegularFile
	}

	f, err := pathrs.Reopen(h, unix.O_RDONLY|unix.O_CLOEXEC)
	if err != nil {
		return nil, err
	}
	name := filepath.Join(root, path)
	return &limitedFile{File: f, lr: &io.LimitedReader{R: f, N: maxUserFileBytes + 1}, name: name}, nil
}

func LookupUser(rootFsPath, userName string) (*ExecUser, error) {
	defaultUser := &user.ExecUser{Uid: DefaultUID, Gid: DefaultGID, Home: DefaultHome}

	var passwd, group io.Reader // stay nil interfaces unless a file was opened

	passwdFile, err := openUserFileInRoot(rootFsPath, "/etc/passwd")
	switch {
	case err == nil:
		passwd = passwdFile
		defer passwdFile.Close()
	case errors.Is(err, os.ErrNotExist) && userName == "root":
		return &ExecUser{Uid: DefaultUID, Gid: DefaultGID, Home: DefaultHome, Sgids: []int{}}, nil
	}
	// any other passwd error: passwd stays nil -> today's "unable to find user X" / numeric uid succeeds

	if groupFile, err := openUserFileInRoot(rootFsPath, "/etc/group"); err == nil {
		group = groupFile
		defer groupFile.Close()
	}

	execUser, err := user.GetExecUser(userName, defaultUser, passwd, group)
	if err != nil {
		return nil, err
	}
	return &ExecUser{Uid: execUser.Uid, Gid: execUser.Gid, Home: execUser.Home, Sgids: execUser.Sgids}, nil
}
