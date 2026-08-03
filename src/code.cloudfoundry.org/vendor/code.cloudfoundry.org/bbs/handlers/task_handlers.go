package handlers

import (
	"context"
	"net/http"
	"time"

	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/lager/v3"
)

//counterfeiter:generate -o fake_controllers/fake_task_controller.go . TaskController

type TaskController interface {
	Tasks(ctx context.Context, logger lager.Logger, domain, cellId string) ([]*models.Task, error)
	TaskByGuid(ctx context.Context, logger lager.Logger, taskGuid string) (*models.Task, error)
	DesireTask(ctx context.Context, logger lager.Logger, taskDefinition *models.TaskDefinition, taskGuid, domain string) error
	StartTask(ctx context.Context, logger lager.Logger, taskGuid, cellId string) (shouldStart bool, err error)
	CancelTask(ctx context.Context, logger lager.Logger, taskGuid string) error
	FailTask(ctx context.Context, logger lager.Logger, taskGuid, failureReason string) error
	RejectTask(ctx context.Context, logger lager.Logger, taskGuid, failureReason string) error
	CompleteTask(ctx context.Context, logger lager.Logger, taskGuid, cellId string, failed bool, failureReason, result string) error
	ResolvingTask(ctx context.Context, logger lager.Logger, taskGuid string) error
	DeleteTask(ctx context.Context, logger lager.Logger, taskGuid string) error
	ConvergeTasks(ctx context.Context, logger lager.Logger, kickTaskDuration, expirePendingTaskDuration, expireCompletedTaskDuration time.Duration) error
}

type TaskHandler struct {
	controller TaskController
	exitChan   chan<- struct{}
}

func NewTaskHandler(
	controller TaskController,
	exitChan chan<- struct{},
) *TaskHandler {
	return &TaskHandler{
		controller: controller,
		exitChan:   exitChan,
	}
}

func (h *TaskHandler) commonTasks(logger lager.Logger, targetVersion format.Version, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("tasks").WithTraceInfo(req)

	request := &models.TasksRequest{}
	response := &models.TasksResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err = parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	tasks, err := h.controller.Tasks(req.Context(), logger, request.Domain, request.CellId)

	downgradedTasks := []*models.Task{}
	for _, t := range tasks {
		downgradedTasks = append(downgradedTasks, t.VersionDownTo(targetVersion))
	}
	response.Tasks = downgradedTasks
	response.Error = models.ConvertError(err)
}

func (h *TaskHandler) Tasks_r2(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonTasks(logger, format.V2, w, req)
}

func (h *TaskHandler) Tasks(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonTasks(logger, format.V3, w, req)
}

func (h *TaskHandler) commonTaskByGuid(logger lager.Logger, targetVersion format.Version, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("task-by-guid").WithTraceInfo(req)

	request := &models.TaskByGuidRequest{}
	response := &models.TaskResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err = parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	var task *models.Task
	task, err = h.controller.TaskByGuid(req.Context(), logger, request.TaskGuid)
	if task != nil {
		task = task.VersionDownTo(targetVersion)
	}

	response.Task = task
	response.Error = models.ConvertError(err)
}

func (h *TaskHandler) TaskByGuid_r2(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonTaskByGuid(logger, format.V2, w, req)
}

func (h *TaskHandler) TaskByGuid(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonTaskByGuid(logger, format.V3, w, req)
}

func (h *TaskHandler) DesireTask(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("desire-task").WithTraceInfo(req)

	request := &models.DesireTaskRequest{}
	response := &models.TaskLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err = parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.DesireTask(req.Context(), logger, request.TaskDefinition, request.TaskGuid, request.Domain)
	response.Error = models.ConvertError(err)
}

func (h *TaskHandler) StartTask(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("start-task").WithTraceInfo(req)

	request := &models.StartTaskRequest{}
	response := &models.StartTaskResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err = parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	response.ShouldStart, err = h.controller.StartTask(req.Context(), logger, request.TaskGuid, request.CellId)
	response.Error = models.ConvertError(err)
}

func (h *TaskHandler) CancelTask(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	logger = logger.Session("cancel-task").WithTraceInfo(req)

	request := &models.TaskGuidRequest{}
	response := &models.TaskLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err := parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.CancelTask(trace.ContextWithRequestId(req), logger, request.TaskGuid)
	response.Error = models.ConvertError(err)
}

// Deprecated: do not use
func (h *TaskHandler) FailTask(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("fail-task").WithTraceInfo(req)

	request := &models.FailTaskRequest{}
	response := &models.TaskLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err = parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.FailTask(req.Context(), logger, request.TaskGuid, request.FailureReason)
	response.Error = models.ConvertError(err)
}

func (h *TaskHandler) RejectTask(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("reject-task").WithTraceInfo(req)

	request := &models.RejectTaskRequest{}
	response := &models.TaskLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err = parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.RejectTask(req.Context(), logger, request.TaskGuid, request.RejectionReason)
	response.Error = models.ConvertError(err)
}

func (h *TaskHandler) CompleteTask(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("complete-task").WithTraceInfo(req)

	request := &models.CompleteTaskRequest{}
	response := &models.TaskLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err = parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		logger.Error("failed-parsing-request", err)
		return
	}

	err = h.controller.CompleteTask(req.Context(), logger, request.TaskGuid, request.CellId, request.Failed, request.FailureReason, request.Result)
	response.Error = models.ConvertError(err)
}

func (h *TaskHandler) ResolvingTask(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("resolving-task").WithTraceInfo(req)

	request := &models.TaskGuidRequest{}
	response := &models.TaskLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err = parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.ResolvingTask(req.Context(), logger, request.TaskGuid)
	response.Error = models.ConvertError(err)
}

func (h *TaskHandler) DeleteTask(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("delete-task").WithTraceInfo(req)

	request := &models.TaskGuidRequest{}
	response := &models.TaskLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err = parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.DeleteTask(req.Context(), logger, request.TaskGuid)
	response.Error = models.ConvertError(err)
}
