package models

import (
	"encoding/json"
	"fmt"
	"strconv"
)

func (m *MetricTagValue) Validate() error {
	var validationError ValidationError

	if m.Static != "" && m.Dynamic.Valid() {
		validationError = validationError.Append(ErrInvalidField{"static"})
		validationError = validationError.Append(ErrInvalidField{"dynamic"})
	}

	if m.Static == "" && !m.Dynamic.Valid() {
		validationError = validationError.Append(ErrInvalidField{"static"})
		validationError = validationError.Append(ErrInvalidField{"dynamic"})
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (v MetricTagValue_DynamicValue) Valid() bool {
	switch v {
	case MetricTagDynamicValueIndex:
		return true
	case MetricTagDynamicValueInstanceGuid:
		return true
	default:
		return false
	}
}

func ConvertMetricTags(metricTags map[string]*MetricTagValue, info map[MetricTagValue_DynamicValue]interface{}) (map[string]string, error) {
	tags := make(map[string]string)
	for k, v := range metricTags {
		if v.Dynamic > 0 {
			switch v.Dynamic {
			case MetricTagDynamicValueIndex:
				val, ok := info[MetricTagDynamicValueIndex].(int32)
				if !ok {
					return nil, fmt.Errorf("could not convert value %+v of type %T to int32", info[MetricTagDynamicValueIndex], info[MetricTagDynamicValueIndex])
				}
				tags[k] = strconv.FormatInt(int64(val), 10)
			case MetricTagDynamicValueInstanceGuid:
				val, ok := info[MetricTagDynamicValueInstanceGuid].(string)
				if !ok {
					return nil, fmt.Errorf("could not convert value %+v of type %T to string", info[MetricTagDynamicValueInstanceGuid], info[MetricTagDynamicValueInstanceGuid])
				}
				tags[k] = val
			}
		} else {
			tags[k] = v.Static
		}
	}
	return tags, nil
}

func validateMetricTags(m map[string]*MetricTagValue, metricsGuid string) ValidationError {
	var validationError ValidationError

	for _, v := range m {
		err := v.Validate()
		if err != nil {
			validationError = validationError.Append(err)
		}
	}

	if len(m) > 0 && metricsGuid != "" {
		if source_id, ok := m["source_id"]; !ok || source_id.Static != metricsGuid {
			validationError = validationError.Append(ErrInvalidField{"source_id should match metrics_guid"})
		}
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (v *MetricTagValue_DynamicValue) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}

	*v = MetricTagValue_DynamicValue(MetricTagValue_DynamicValue_value[name])

	return nil
}

func (v MetricTagValue_DynamicValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(MetricTagValue_DynamicValue_name[int32(v)])
}
