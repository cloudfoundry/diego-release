package netns

import "os"

type Opener func(path string) (*os.File, error)

func (o Opener) Open(path string) (*os.File, error) {
	return o(path)
}
