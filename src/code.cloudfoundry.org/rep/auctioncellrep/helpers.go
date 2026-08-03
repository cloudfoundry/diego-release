package auctioncellrep

import uuid "github.com/nu7hatch/gouuid"

func GenerateGuid() (string, error) {
	guid, err := uuid.NewV4()
	if err != nil {
		return "", err
	}

	guidString := guid.String()
	if len(guidString) > 28 {
		guidString = guidString[:28]
	}

	return guidString, nil
}
