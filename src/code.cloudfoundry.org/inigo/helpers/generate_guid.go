package helpers

import uuid "github.com/nu7hatch/gouuid"

func GenerateGuid() string {
	guid, err := uuid.NewV4()
	if err != nil {
		panic("Failed to generate a GUID.  Craziness.")
	}

	return guid.String()
}
