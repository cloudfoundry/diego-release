package fakes

import "sync"

type Log struct {
	AppId          string
	Message        string
	SourceType     string
	SourceInstance string
	MessageType    string
}

type FakeLogSender struct {
	FakeClient
	logs     []Log
	logsLock sync.Mutex
}

const (
	OUT = "OUT"
	ERR = "ERR"
)

func NewFakeLogSender() *FakeLogSender {
	sender := &FakeLogSender{}
	sender.FakeClient.SendAppErrorLogStub = func(appId, message, sourceType, sourceInstance string) error {
		sender.logsLock.Lock()
		defer sender.logsLock.Unlock()
		sender.logs = append(sender.logs, Log{
			AppId:          appId,
			Message:        message,
			SourceType:     sourceType,
			SourceInstance: sourceInstance,
			MessageType:    ERR,
		})
		return nil
	}
	sender.FakeClient.SendAppLogStub = func(appId, message, sourceType, sourceInstance string) error {
		sender.logsLock.Lock()
		defer sender.logsLock.Unlock()
		sender.logs = append(sender.logs, Log{
			AppId:          appId,
			Message:        message,
			SourceType:     sourceType,
			SourceInstance: sourceInstance,
			MessageType:    OUT,
		})
		return nil
	}
	return sender
}

func (sender *FakeLogSender) Logs() []Log {
	sender.logsLock.Lock()
	defer sender.logsLock.Unlock()
	logsCopy := make([]Log, len(sender.logs))
	copy(logsCopy, sender.logs)
	return logsCopy
}
