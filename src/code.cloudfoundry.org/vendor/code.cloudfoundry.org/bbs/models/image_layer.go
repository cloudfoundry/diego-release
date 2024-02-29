package models

import (
	"encoding/json"
	"fmt"
	strings "strings"
)

func (l *ImageLayer) Validate() error {
	var validationError ValidationError

	if l.GetUrl() == "" {
		validationError = validationError.Append(ErrInvalidField{"url"})
	}

	if l.GetDestinationPath() == "" {
		validationError = validationError.Append(ErrInvalidField{"destination_path"})
	}

	if !l.LayerType.Valid() {
		validationError = validationError.Append(ErrInvalidField{"layer_type"})
	}

	if !l.MediaType.Valid() {
		validationError = validationError.Append(ErrInvalidField{"media_type"})
	}

	if (l.DigestValue != "" || l.LayerType == LayerTypeExclusive) && l.DigestAlgorithm == DigestAlgorithmInvalid {
		validationError = validationError.Append(ErrInvalidField{"digest_algorithm"})
	}

	if (l.DigestAlgorithm != DigestAlgorithmInvalid || l.LayerType == LayerTypeExclusive) && l.DigestValue == "" {
		validationError = validationError.Append(ErrInvalidField{"digest_value"})
	}

	if l.DigestValue != "" && !l.DigestAlgorithm.Valid() {
		validationError = validationError.Append(ErrInvalidField{"digest_algorithm"})
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func validateImageLayers(layers []*ImageLayer, legacyDownloadUser string) ValidationError {
	var validationError ValidationError

	requiresLegacyDownloadUser := false
	if len(layers) > 0 {
		for _, layer := range layers {
			err := layer.Validate()
			if err != nil {
				validationError = validationError.Append(ErrInvalidField{"image_layer"})
				validationError = validationError.Append(err)
			}

			if layer.LayerType == LayerTypeExclusive {
				requiresLegacyDownloadUser = true
			}
		}
	}

	if requiresLegacyDownloadUser && legacyDownloadUser == "" {
		validationError = validationError.Append(ErrInvalidField{"legacy_download_user"})
	}

	return validationError
}

type ImageLayers []*ImageLayer

func (layers ImageLayers) FilterByType(layerType ImageLayer_Type) ImageLayers {
	var filtered ImageLayers

	for _, layer := range layers {
		if layer.GetLayerType() == layerType {
			filtered = append(filtered, layer)
		}
	}
	return filtered
}

func (layers ImageLayers) ToDownloadActions(legacyDownloadUser string, existingAction *Action) *Action {
	downloadActions := []ActionInterface{}

	for _, layer := range layers.FilterByType(LayerTypeExclusive) {
		digestAlgorithmName := strings.ToLower(layer.DigestAlgorithm.String())
		downloadActions = append(downloadActions, &DownloadAction{
			Artifact:          layer.Name,
			From:              layer.Url,
			To:                layer.DestinationPath,
			CacheKey:          digestAlgorithmName + ":" + layer.DigestValue, // digest required for exclusive layers
			User:              legacyDownloadUser,
			ChecksumAlgorithm: digestAlgorithmName,
			ChecksumValue:     layer.DigestValue,
		})
	}

	var action *Action
	if len(downloadActions) > 0 {
		parallelDownloadActions := Parallel(downloadActions...)
		if existingAction != nil {
			action = WrapAction(Serial(parallelDownloadActions, UnwrapAction(existingAction)))
		} else {
			action = WrapAction(parallelDownloadActions)
		}
	} else {
		action = existingAction
	}

	return action
}

func (layers ImageLayers) ToCachedDependencies() []*CachedDependency {
	cachedDependencies := []*CachedDependency{}
	for _, layer := range layers.FilterByType(LayerTypeShared) {
		c := &CachedDependency{
			Name:          layer.Name,
			From:          layer.Url,
			To:            layer.DestinationPath,
			ChecksumValue: layer.DigestValue,
		}

		if layer.DigestAlgorithm == DigestAlgorithmInvalid {
			c.ChecksumAlgorithm = ""
		} else {
			c.ChecksumAlgorithm = strings.ToLower(layer.DigestAlgorithm.String())
		}

		if layer.DigestValue == "" {
			c.CacheKey = layer.Url
		} else {
			c.CacheKey = c.ChecksumAlgorithm + ":" + layer.DigestValue
		}

		cachedDependencies = append(cachedDependencies, c)
	}

	return cachedDependencies
}

func (d ImageLayer_DigestAlgorithm) Valid() bool {
	switch d {
	case DigestAlgorithmSha256:
		return true
	case DigestAlgorithmSha512:
		return true
	default:
		return false
	}
}

func (m ImageLayer_MediaType) Valid() bool {
	switch m {
	case MediaTypeTar:
		return true
	case MediaTypeTgz:
		return true
	case MediaTypeZip:
		return true
	default:
		return false
	}
}

func (t ImageLayer_Type) Valid() bool {
	switch t {
	case LayerTypeExclusive:
		return true
	case LayerTypeShared:
		return true
	default:
		return false
	}
}

func (d *ImageLayer_DigestAlgorithm) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}

	if v, found := ImageLayer_DigestAlgorithm_value[name]; found {
		*d = ImageLayer_DigestAlgorithm(v)
		return nil
	}
	return fmt.Errorf("invalid digest_algorithm: %s", name)
}

func (d ImageLayer_DigestAlgorithm) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (m *ImageLayer_MediaType) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}

	if v, found := ImageLayer_MediaType_value[name]; found {
		*m = ImageLayer_MediaType(v)
		return nil
	}
	return fmt.Errorf("invalid media_type: %s", name)
}

func (m ImageLayer_MediaType) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.String())
}

func (t *ImageLayer_Type) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}

	if v, found := ImageLayer_Type_value[name]; found {
		*t = ImageLayer_Type(v)
		return nil
	}
	return fmt.Errorf("invalid type: %s", name)
}

func (t ImageLayer_Type) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}
