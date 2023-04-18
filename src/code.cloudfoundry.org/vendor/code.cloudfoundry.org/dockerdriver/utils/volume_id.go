package utils

import (
	"fmt"
	"strings"
)

type VolumeId struct {
	Prefix string `json:"prefix"`
	Suffix string `json:"suffix"`
}

func NewVolumeId(prefix, suffix string) VolumeId {
	return VolumeId{
		Prefix: prefix,
		Suffix: suffix,
	}
}

func NewVolumeIdFromEncodedString(volumeString string) (VolumeId, error) {
	parts := strings.Split(volumeString, "_")

	if len(parts) != 2 {
		return VolumeId{}, fmt.Errorf("Incorrectly encoded volume ID string: %q", volumeString)
	}

	return VolumeId{Prefix: strings.Replace(parts[0], "=", "_", -1), Suffix: strings.Replace(parts[1], "=", "_", -1)}, nil
}

func (id VolumeId) GetUniqueId() string {
	return fmt.Sprintf("%s_%s", strings.Replace(id.Prefix, "_", "=", -1), strings.Replace(id.Suffix, "_", "=", -1))
}
