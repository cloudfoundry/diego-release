package serviceclient

import (
	"encoding/json"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"context"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
	locketmodels "code.cloudfoundry.org/locket/models"
)

const BBSLockSchemaKey = "bbs_lock"

//go:generate counterfeiter -generate

//counterfeiter:generate . ServiceClient

type ServiceClient interface {
	CellById(logger lager.Logger, cellId string) (*models.CellPresence, error)
	Cells(logger lager.Logger) (models.CellSet, error)
	CellEvents(logger lager.Logger) <-chan models.CellEvent
}

type serviceClient struct {
	locketClient      locketmodels.LocketClient
	connectionTimeout time.Duration
}

func NewServiceClient(locketClient locketmodels.LocketClient, connectionTimeout time.Duration) *serviceClient {
	return &serviceClient{
		locketClient:      locketClient,
		connectionTimeout: connectionTimeout,
	}
}

func (s *serviceClient) Cells(logger lager.Logger) (models.CellSet, error) {
	logger = logger.Session("cells")

	var cellSet = models.CellSet{}
	ctx, cancel := context.WithTimeout(context.Background(), s.connectionTimeout)
	resp, err := s.locketClient.FetchAll(ctx, &locketmodels.FetchAllRequest{Type: locketmodels.PresenceType, TypeCode: locketmodels.PRESENCE})
	defer cancel()
	if err != nil {
		logger.Error("failed-to-fetch-cells-from-locket", err)
		return nil, err
	}

	for _, resource := range resp.Resources {
		presence, err := presenceFromResource(resource)
		if err != nil {
			logger.Error("failed-to-unmarshal-presence", err)
			continue
		}
		cellSet.Add(presence)
	}

	return cellSet, nil
}

func (s *serviceClient) CellById(logger lager.Logger, cellId string) (*models.CellPresence, error) {
	logger = logger.Session("cell-by-id", lager.Data{"cell-id": cellId})
	var presence *models.CellPresence
	ctx, cancel := context.WithTimeout(context.Background(), s.connectionTimeout)
	resp, locketErr := s.locketClient.Fetch(ctx, &locketmodels.FetchRequest{
		Key: cellId,
	})
	defer cancel()
	if locketErr != nil {
		logger.Error("failed-to-fetch-presence-from-locket", locketErr)
		if status.Code(locketErr) == codes.NotFound {
			return nil, models.ErrResourceNotFound
		}
		return nil, locketErr
	}

	var err error
	presence, err = presenceFromResource(resp.Resource)
	if err != nil {
		logger.Error("failed-to-unmarshal-presence", err)
		return nil, err
	}

	return presence, nil
}

func (s *serviceClient) CellEvents(logger lager.Logger) <-chan models.CellEvent {
	return nil
}

func presenceFromResource(resource *locketmodels.Resource) (*models.CellPresence, error) {
	cellPresence := &models.CellPresence{}
	err := json.Unmarshal([]byte(resource.Value), cellPresence)
	return cellPresence, err
}
