package goci

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type BndlLoader struct {
}

func (b *BndlLoader) Load(path string) (Bndl, error) {
	bundle := Bndl{}
	err := readJSONInto(filepath.Join(path, "config.json"), &bundle.Spec)
	if err != nil {
		return bundle, fmt.Errorf("Failed to load bundle: %s", err)
	}

	return bundle, nil
}

type BundleSaver struct{}

func (b BundleSaver) Save(bundle Bndl, path string) error {
	return save(bundle.Spec, filepath.Join(path, "config.json"))
}

func save(value interface{}, path string) error {
	w, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("Failed to save bundle: %s", err)
	}
	defer w.Close()

	return json.NewEncoder(w).Encode(value)
}

func readJSONInto(path string, object interface{}) error {
	runtimeContents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(runtimeContents, object)
}
