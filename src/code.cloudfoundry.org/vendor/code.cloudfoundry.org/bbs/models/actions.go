package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/format"
	proto "github.com/gogo/protobuf/proto"
)

const (
	ActionTypeDownload     = "download"
	ActionTypeEmitProgress = "emit_progress"
	ActionTypeRun          = "run"
	ActionTypeUpload       = "upload"
	ActionTypeTimeout      = "timeout"
	ActionTypeTry          = "try"
	ActionTypeParallel     = "parallel"
	ActionTypeSerial       = "serial"
	ActionTypeCodependent  = "codependent"
)

var ErrInvalidActionType = errors.New("invalid action type")

type ActionInterface interface {
	ActionType() string
	Validate() error
	proto.Message
}

func (a *Action) GetValue() interface{} {
	if a.DownloadAction != nil {
		return a.DownloadAction
	}
	if a.UploadAction != nil {
		return a.UploadAction
	}
	if a.RunAction != nil {
		return a.RunAction
	}
	if a.TimeoutAction != nil {
		return a.TimeoutAction
	}
	if a.EmitProgressAction != nil {
		return a.EmitProgressAction
	}
	if a.TryAction != nil {
		return a.TryAction
	}
	if a.ParallelAction != nil {
		return a.ParallelAction
	}
	if a.SerialAction != nil {
		return a.SerialAction
	}
	if a.CodependentAction != nil {
		return a.CodependentAction
	}
	return nil
}

func (a *Action) SetValue(value interface{}) bool {
	switch vt := value.(type) {
	case *DownloadAction:
		a.DownloadAction = vt
	case *UploadAction:
		a.UploadAction = vt
	case *RunAction:
		a.RunAction = vt
	case *TimeoutAction:
		a.TimeoutAction = vt
	case *EmitProgressAction:
		a.EmitProgressAction = vt
	case *TryAction:
		a.TryAction = vt
	case *ParallelAction:
		a.ParallelAction = vt
	case *SerialAction:
		a.SerialAction = vt
	case *CodependentAction:
		a.CodependentAction = vt
	default:
		return false
	}
	return true
}

func (a *Action) Validate() error {
	if a == nil {
		return nil
	}

	if inner := UnwrapAction(a); inner != nil {
		err := inner.Validate()
		if err != nil {
			return err
		}
	} else {
		return ErrInvalidField{"inner-action"}
	}
	return nil
}

func (a *DownloadAction) ActionType() string {
	return ActionTypeDownload
}

func (a DownloadAction) Validate() error {
	var validationError ValidationError

	if a.GetFrom() == "" {
		validationError = validationError.Append(ErrInvalidField{"from"})
	}

	if a.GetTo() == "" {
		validationError = validationError.Append(ErrInvalidField{"to"})
	}

	if a.GetUser() == "" {
		validationError = validationError.Append(ErrInvalidField{"user"})
	}

	if a.GetChecksumValue() != "" && a.GetChecksumAlgorithm() == "" {
		validationError = validationError.Append(ErrInvalidField{"checksum algorithm"})
	}

	if a.GetChecksumValue() == "" && a.GetChecksumAlgorithm() != "" {
		validationError = validationError.Append(ErrInvalidField{"checksum value"})
	}

	if a.GetChecksumValue() != "" && a.GetChecksumAlgorithm() != "" {
		if !contains([]string{"md5", "sha1", "sha256"}, strings.ToLower(a.GetChecksumAlgorithm())) {
			validationError = validationError.Append(ErrInvalidField{"invalid algorithm"})
		}
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func contains(array []string, element string) bool {
	for _, item := range array {
		if item == element {
			return true
		}
	}
	return false
}

func (a *UploadAction) ActionType() string {
	return ActionTypeUpload
}

func (a UploadAction) Validate() error {
	var validationError ValidationError

	if a.GetTo() == "" {
		validationError = validationError.Append(ErrInvalidField{"to"})
	}

	if a.GetFrom() == "" {
		validationError = validationError.Append(ErrInvalidField{"from"})
	}

	if a.GetUser() == "" {
		validationError = validationError.Append(ErrInvalidField{"user"})
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (a *RunAction) ActionType() string {
	return ActionTypeRun
}

func (a RunAction) Validate() error {
	var validationError ValidationError

	if a.Path == "" {
		validationError = validationError.Append(ErrInvalidField{"path"})
	}

	if a.User == "" {
		validationError = validationError.Append(ErrInvalidField{"user"})
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (a *TimeoutAction) ActionType() string {
	return ActionTypeTimeout
}

func (a TimeoutAction) Validate() error {
	var validationError ValidationError

	if a.Action == nil {
		validationError = validationError.Append(ErrInvalidField{"action"})
	} else {
		err := UnwrapAction(a.Action).Validate()
		if err != nil {
			validationError = validationError.Append(err)
		}
	}

	if a.GetTimeoutMs() <= 0 {
		validationError = validationError.Append(ErrInvalidField{"timeout_ms"})
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (a *TryAction) ActionType() string {
	return ActionTypeTry
}

func (a TryAction) Validate() error {
	var validationError ValidationError

	if a.Action == nil {
		validationError = validationError.Append(ErrInvalidField{"action"})
	} else {
		err := UnwrapAction(a.Action).Validate()
		if err != nil {
			validationError = validationError.Append(err)
		}
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (*ParallelAction) Version() format.Version {
	return format.V0
}

func (a *ParallelAction) ActionType() string {
	return ActionTypeParallel
}

func (a ParallelAction) Validate() error {
	var validationError ValidationError

	if a.Actions == nil || len(a.Actions) == 0 {
		validationError = validationError.Append(ErrInvalidField{"actions"})
	} else {
		for index, action := range a.Actions {
			if action == nil {
				errorString := fmt.Sprintf("action at index %d", index)
				validationError = validationError.Append(ErrInvalidField{errorString})
			} else {
				err := UnwrapAction(action).Validate()
				if err != nil {
					validationError = validationError.Append(err)
				}
			}
		}
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (a *CodependentAction) ActionType() string {
	return ActionTypeCodependent
}

func (a CodependentAction) Validate() error {
	var validationError ValidationError

	if a.Actions == nil || len(a.Actions) == 0 {
		validationError = validationError.Append(ErrInvalidField{"actions"})
	} else {
		for index, action := range a.Actions {
			if action == nil {
				errorString := fmt.Sprintf("action at index %d", index)
				validationError = validationError.Append(ErrInvalidField{errorString})
			} else {
				err := UnwrapAction(action).Validate()
				if err != nil {
					validationError = validationError.Append(err)
				}
			}
		}
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

// func (*SerialAction) Version() format.Version {
// 	return format.V0
// }

// func (*SerialAction) MigrateFromVersion(v format.Version) error {
// 	return nil
// }

func (a *SerialAction) ActionType() string {
	return ActionTypeSerial
}

func (a SerialAction) Validate() error {
	var validationError ValidationError

	if a.Actions == nil || len(a.Actions) == 0 {
		validationError = validationError.Append(ErrInvalidField{"actions"})
	} else {
		for index, action := range a.Actions {
			if action == nil {
				errorString := fmt.Sprintf("action at index %d", index)
				validationError = validationError.Append(ErrInvalidField{errorString})
			} else {
				err := UnwrapAction(action).Validate()
				if err != nil {
					validationError = validationError.Append(err)
				}
			}
		}
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (a *EmitProgressAction) ActionType() string {
	return ActionTypeEmitProgress
}

func (a EmitProgressAction) Validate() error {
	var validationError ValidationError

	if a.Action == nil {
		validationError = validationError.Append(ErrInvalidField{"action"})
	} else {
		err := UnwrapAction(a.Action).Validate()
		if err != nil {
			validationError = validationError.Append(err)
		}
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func EmitProgressFor(action ActionInterface, startMessage string, successMessage string, failureMessagePrefix string) *EmitProgressAction {
	return &EmitProgressAction{
		Action:               WrapAction(action),
		StartMessage:         startMessage,
		SuccessMessage:       successMessage,
		FailureMessagePrefix: failureMessagePrefix,
	}
}

func Timeout(action ActionInterface, timeout time.Duration) *TimeoutAction {
	return &TimeoutAction{
		Action:    WrapAction(action),
		TimeoutMs: (int64)(timeout / 1000000),
	}
}

func Try(action ActionInterface) *TryAction {
	return &TryAction{Action: WrapAction(action)}
}

func Parallel(actions ...ActionInterface) *ParallelAction {
	return &ParallelAction{Actions: WrapActions(actions)}
}

func Codependent(actions ...ActionInterface) *CodependentAction {
	return &CodependentAction{Actions: WrapActions(actions)}
}

func Serial(actions ...ActionInterface) *SerialAction {
	return &SerialAction{Actions: WrapActions(actions)}
}

func UnwrapAction(action *Action) ActionInterface {
	if action == nil {
		return nil
	}
	a := action.GetValue()
	if a == nil {
		return nil
	}
	return a.(ActionInterface)
}

func WrapActions(actions []ActionInterface) []*Action {
	wrappedActions := make([]*Action, 0, len(actions))
	for _, action := range actions {
		wrappedActions = append(wrappedActions, WrapAction(action))
	}
	return wrappedActions
}

func WrapAction(action ActionInterface) *Action {
	if action == nil {
		return nil
	}
	a := &Action{}
	a.SetValue(action)
	return a
}

// SetDeprecatedTimeoutNs returns a deep copy of the Action tree.  If there are
// any TimeoutActions in the tree, their DeprecatedStartTimeoutS is set to
// `TimeoutMs * time.Millisecond'.
func (action *Action) SetDeprecatedTimeoutNs() *Action {
	if action == nil {
		return nil
	}

	a := action.GetValue()
	switch actionModel := a.(type) {
	case *RunAction, *DownloadAction, *UploadAction:
		return action

	case *TimeoutAction:
		timeoutAction := *actionModel
		timeoutAction.DeprecatedTimeoutNs = timeoutAction.TimeoutMs * int64(time.Millisecond)
		return WrapAction(&timeoutAction)

	case *EmitProgressAction:
		return actionModel.Action.SetDeprecatedTimeoutNs()

	case *TryAction:
		return actionModel.Action.SetDeprecatedTimeoutNs()

	case *ParallelAction:
		newActions := []*Action{}
		for _, subaction := range actionModel.Actions {
			newActions = append(newActions, subaction.SetDeprecatedTimeoutNs())
		}
		parallelAction := *actionModel
		parallelAction.Actions = newActions
		return WrapAction(&parallelAction)

	case *SerialAction:
		newActions := []*Action{}
		for _, subaction := range actionModel.Actions {
			newActions = append(newActions, subaction.SetDeprecatedTimeoutNs())
		}
		serialAction := *actionModel
		serialAction.Actions = newActions
		return WrapAction(&serialAction)

	case *CodependentAction:
		newActions := []*Action{}
		for _, subaction := range actionModel.Actions {
			newActions = append(newActions, subaction.SetDeprecatedTimeoutNs())
		}
		codependentAction := *actionModel
		codependentAction.Actions = newActions
		return WrapAction(&codependentAction)
	}

	return action
}

func (action *Action) SetTimeoutMsFromDeprecatedTimeoutNs() {
	if action == nil {
		return
	}

	a := action.GetValue()
	switch actionModel := a.(type) {
	case *RunAction, *DownloadAction, *UploadAction:
		return

	case *TimeoutAction:
		timeoutAction := actionModel
		timeoutAction.TimeoutMs = timeoutAction.DeprecatedTimeoutNs / int64(time.Millisecond)

	case *EmitProgressAction:
		actionModel.Action.SetDeprecatedTimeoutNs()

	case *TryAction:
		actionModel.Action.SetDeprecatedTimeoutNs()

	case *ParallelAction:
		for _, subaction := range actionModel.Actions {
			subaction.SetDeprecatedTimeoutNs()
		}

	case *SerialAction:
		for _, subaction := range actionModel.Actions {
			subaction.SetDeprecatedTimeoutNs()
		}

	case *CodependentAction:
		for _, subaction := range actionModel.Actions {
			subaction.SetDeprecatedTimeoutNs()
		}
	}
}

type internalResourceLimits struct {
	Nofile *uint64 `json:"nofile,omitempty"`
	Nproc  *uint64 `json:"nproc,omitempty"`
}

func (l *ResourceLimits) UnmarshalJSON(data []byte) error {
	var limit internalResourceLimits
	if err := json.Unmarshal(data, &limit); err != nil {
		return err
	}

	if limit.Nofile != nil {
		l.SetNofile(*limit.Nofile)
	}
	if limit.Nproc != nil {
		l.SetNproc(*limit.Nproc)
	}

	return nil
}

func (l ResourceLimits) MarshalJSON() ([]byte, error) {
	var limit internalResourceLimits
	if l.NofileExists() {
		n := l.GetNofile()
		limit.Nofile = &n
	}
	if l.NprocExists() {
		n := l.GetNproc()
		limit.Nproc = &n
	}
	return json.Marshal(limit)
}

func (l *ResourceLimits) SetNofile(nofile uint64) {
	l.OptionalNofile = &ResourceLimits_Nofile{
		Nofile: nofile,
	}
}

func (m *ResourceLimits) GetNofilePtr() *uint64 {
	if x, ok := m.GetOptionalNofile().(*ResourceLimits_Nofile); ok {
		return &x.Nofile
	}
	return nil
}

func (l *ResourceLimits) NofileExists() bool {
	_, ok := l.GetOptionalNofile().(*ResourceLimits_Nofile)
	return ok
}

func (l *ResourceLimits) SetNproc(nproc uint64) {
	l.OptionalNproc = &ResourceLimits_Nproc{
		Nproc: nproc,
	}
}

func (l *ResourceLimits) NprocExists() bool {
	_, ok := l.GetOptionalNproc().(*ResourceLimits_Nproc)
	return ok
}
