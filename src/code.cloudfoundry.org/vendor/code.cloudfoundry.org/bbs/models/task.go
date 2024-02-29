package models

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"

	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/lager/v3"
)

var taskGuidPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type TaskChange struct {
	Before *Task
	After  *Task
}

type TaskFilter struct {
	Domain string
	CellID string
}

func (t *Task) LagerData() lager.Data {
	return lager.Data{
		"task_guid": t.TaskGuid,
		"domain":    t.Domain,
		"state":     t.State,
		"cell_id":   t.CellId,
	}
}

func (task *Task) Validate() error {
	var validationError ValidationError

	if task.Domain == "" {
		validationError = validationError.Append(ErrInvalidField{"domain"})
	}

	if !taskGuidPattern.MatchString(task.TaskGuid) {
		validationError = validationError.Append(ErrInvalidField{"task_guid"})
	}

	if task.TaskDefinition == nil {
		validationError = validationError.Append(ErrInvalidField{"task_definition"})
	} else if defErr := task.TaskDefinition.Validate(); defErr != nil {
		validationError = validationError.Append(defErr)
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (t *Task) Copy() *Task {
	newTask := *t
	newTask.TaskDefinition = t.TaskDefinition.Copy()
	return &newTask
}

func (t *Task) ValidateTransitionTo(to Task_State) error {
	var valid bool
	from := t.State
	switch to {
	case Task_Running:
		valid = from == Task_Pending
	case Task_Completed:
		valid = from == Task_Running
	case Task_Resolving:
		valid = from == Task_Completed
	}

	if !valid {
		return NewError(
			Error_InvalidStateTransition,
			fmt.Sprintf("Cannot transition from %s to %s", from.String(), to.String()),
		)
	}

	return nil
}

func (t *TaskDefinition) Copy() *TaskDefinition {
	if t == nil {
		return &TaskDefinition{}
	}
	newTaskDef := *t
	return &newTaskDef
}

func (def *TaskDefinition) Validate() error {
	var validationError ValidationError

	if def.RootFs == "" {
		validationError = validationError.Append(ErrInvalidField{"rootfs"})
	} else {
		rootFsURL, err := url.Parse(def.RootFs)
		if err != nil || rootFsURL.Scheme == "" {
			validationError = validationError.Append(ErrInvalidField{"rootfs"})
		}
	}

	if def.Action == nil {
		validationError = validationError.Append(ErrInvalidActionType)
	} else if err := def.Action.Validate(); err != nil {
		validationError = validationError.Append(ErrInvalidField{"action"})
		validationError = validationError.Append(err)
	}

	if def.MemoryMb < 0 {
		validationError = validationError.Append(ErrInvalidField{"memory_mb"})
	}

	if def.DiskMb < 0 {
		validationError = validationError.Append(ErrInvalidField{"disk_mb"})
	}

	if limit := def.LogRateLimit; limit != nil {
		if limit.BytesPerSecond < -1 {
			validationError = validationError.Append(ErrInvalidField{"log_rate_limit"})
		}
	}

	if def.MaxPids < 0 {
		validationError = validationError.Append(ErrInvalidField{"max_pids"})
	}

	if len(def.Annotation) > maximumAnnotationLength {
		validationError = validationError.Append(ErrInvalidField{"annotation"})
	}

	for _, rule := range def.EgressRules {
		err := rule.Validate()
		if err != nil {
			validationError = validationError.Append(ErrInvalidField{"egress_rules"})
		}
	}

	if def.ImageUsername == "" && def.ImagePassword != "" {
		validationError = validationError.Append(ErrInvalidField{"image_username"})
	}

	if def.ImageUsername != "" && def.ImagePassword == "" {
		validationError = validationError.Append(ErrInvalidField{"image_password"})
	}

	err := validateCachedDependencies(def.CachedDependencies)
	if err != nil {
		validationError = validationError.Append(err)
	}

	err = validateImageLayers(def.ImageLayers, def.LegacyDownloadUser)
	if err != nil {
		validationError = validationError.Append(err)
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func downgradeTaskDefinitionV3ToV2(t *TaskDefinition) *TaskDefinition {
	layers := ImageLayers(t.ImageLayers)

	t.CachedDependencies = append(layers.ToCachedDependencies(), t.CachedDependencies...)
	t.Action = layers.ToDownloadActions(t.LegacyDownloadUser, t.Action)
	t.ImageLayers = nil

	return t
}

func (t *Task) VersionDownTo(v format.Version) *Task {
	t = t.Copy()

	if v < t.Version() {
		t.TaskDefinition = downgradeTaskDefinitionV3ToV2(t.TaskDefinition)
	}

	return t
}

func (t *Task) Version() format.Version {
	return format.V3
}

func (s *Task_State) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}

	if v, found := Task_State_value[name]; found {
		*s = Task_State(v)
		return nil
	}
	return fmt.Errorf("invalid state: %s", name)
}

func (s Task_State) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}
