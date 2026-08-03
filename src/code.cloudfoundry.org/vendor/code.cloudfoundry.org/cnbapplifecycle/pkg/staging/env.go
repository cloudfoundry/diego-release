package staging

import (
	"fmt"
	"os"
	"path/filepath"
)

func CreateEnvFiles(platformDir string, envKeys []string) error {
	envDir := filepath.Join(platformDir, "env")
	err := os.MkdirAll(envDir, 0o755)
	if err != nil {
		return err
	}

	for _, k := range envKeys {
		val, ok := os.LookupEnv(k)
		if !ok {
			return fmt.Errorf("requested environment variable %q not found", k)
		}

		if err := os.WriteFile(filepath.Join(envDir, k), []byte(val), 0o644); err != nil {
			return err
		}
	}

	return nil
}
