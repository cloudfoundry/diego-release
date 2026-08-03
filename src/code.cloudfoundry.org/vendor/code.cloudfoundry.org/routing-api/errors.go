package routing_api

type Type string
type Error struct {
	Type    Type   `json:"name"`
	Message string `json:"message"`
}

func (err Error) Error() string {
	return err.Message
}

func NewError(errType Type, message string) Error {
	return Error{
		Type:    errType,
		Message: message,
	}
}

const (
	ResponseError               Type = "ResponseError"
	ResourceNotFoundError       Type = "ResourceNotFoundError"
	ProcessRequestError         Type = "ProcessRequestError"
	RouteInvalidError           Type = "RouteInvalidError"
	RouteServiceUrlInvalidError Type = "RouteServiceUrlInvalidError"
	DBCommunicationError        Type = "DBCommunicationError"
	GuidGenerationError         Type = "GuidGenerationError"
	UnauthorizedError           Type = "UnauthorizedError"
	TcpRouteMappingInvalidError Type = "TcpRouteMappingInvalidError"
	DBConflictError             Type = "DBConflictError"
	PortRangeExhaustedError     Type = "PortRangeExhaustedError"
)
