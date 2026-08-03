package env

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"code.cloudfoundry.org/buildpackapplifecycle/credhub"
	"code.cloudfoundry.org/buildpackapplifecycle/databaseuri"
	"code.cloudfoundry.org/buildpackapplifecycle/platformoptions"
	"code.cloudfoundry.org/goshims/osshim"
)

func CalcEnv(os osshim.Os, dir string, attempts int, delay time.Duration) error {
	err := os.Setenv("HOME", dir)
	if err != nil {
		return fmt.Errorf("Unable to set HOME environment variable: %v", err)
	}

	tmpDir, err := filepath.Abs(filepath.Join(dir, "..", "tmp"))
	if err == nil {
		err = os.Setenv("TMPDIR", tmpDir)
		if err != nil {
			return fmt.Errorf("Unable to set TMPDIR environment variable: %v", err)
		}
	}

	depsDir, err := filepath.Abs(filepath.Join(dir, "..", "deps"))
	if err == nil {
		err = os.Setenv("DEPS_DIR", depsDir)
		if err != nil {
			return fmt.Errorf("Unable to set DEPS_DIR environment variable: %v", err)
		}
	}

	vcapAppEnv := map[string]interface{}{}
	err = json.Unmarshal([]byte(os.Getenv("VCAP_APPLICATION")), &vcapAppEnv)
	if err == nil {
		vcapAppEnv["host"] = "0.0.0.0"

		vcapAppEnv["instance_id"] = os.Getenv("INSTANCE_GUID")

		port, err := strconv.Atoi(os.Getenv("PORT"))
		if err == nil {
			vcapAppEnv["port"] = port
		}

		index, err := strconv.Atoi(os.Getenv("INSTANCE_INDEX"))
		if err == nil {
			vcapAppEnv["instance_index"] = index
		}

		mungedAppEnv, err := json.Marshal(vcapAppEnv)
		if err == nil {
			err = os.Setenv("VCAP_APPLICATION", string(mungedAppEnv))
			if err != nil {
				return fmt.Errorf("Unable to set VCAP_APPLICATION environment variable: %v", err)
			}
		}
	}

	if platformOptions, err := platformoptions.Get(os.Getenv("VCAP_PLATFORM_OPTIONS")); err != nil {
		return fmt.Errorf("Invalid platform options: %v", err)
	} else if platformOptions != nil && platformOptions.CredhubURI != "" {
		err := credhub.New(&osshim.OsShim{}, attempts, delay).InterpolateServiceRefs(platformOptions.CredhubURI)
		if err != nil {
			return fmt.Errorf("Unable to interpolate credhub refs: %v", err)
		}
	}
	err = os.Unsetenv("VCAP_PLATFORM_OPTIONS")
	if err != nil {
		return fmt.Errorf("Unable to unset VCAP_PLATFORM_OPTIONS environment variable: %v", err)
	}

	if os.Getenv("VCAP_SERVICES") != "" {
		dbUri := databaseuri.New()
		if creds, err := dbUri.Credentials([]byte(os.Getenv("VCAP_SERVICES"))); err == nil {
			databaseUrl := dbUri.Uri(creds)
			if databaseUrl != "" {
				err = os.Setenv("DATABASE_URL", databaseUrl)
				if err != nil {
					return fmt.Errorf("Unable to set DATABASE_URL environment variable: %v", err)
				}
			}
		}
	}

	return nil
}
