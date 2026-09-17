package properties

import (
	"encoding/json"
	"os"
)

func Load(path string) (*Manager, error) {
	f, err := os.Open(path)
	if err != nil {
		return NewManager(), nil
	}
	defer f.Close()

	fileInfo, err := f.Stat()
	if err != nil {
		return nil, err
	}

	if fileInfo.Size() == 0 {
		return NewManager(), nil
	}

	var mgr Manager
	if err := json.NewDecoder(f).Decode(&mgr); err != nil {
		return nil, err
	}

	return &mgr, nil
}

func Save(path string, mgr *Manager) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(mgr)
}
