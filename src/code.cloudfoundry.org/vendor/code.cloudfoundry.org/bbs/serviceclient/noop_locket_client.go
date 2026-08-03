package serviceclient

import (
	"context"

	locketmodels "code.cloudfoundry.org/locket/models"
	"google.golang.org/grpc"
)

type noopLocketClient struct{}

func NewNoopLocketClient() *noopLocketClient {
	return &noopLocketClient{}
}

func (*noopLocketClient) Lock(ctx context.Context, in *locketmodels.LockRequest, opts ...grpc.CallOption) (*locketmodels.LockResponse, error) {
	panic("not implemented")
}

func (*noopLocketClient) Fetch(ctx context.Context, in *locketmodels.FetchRequest, opts ...grpc.CallOption) (*locketmodels.FetchResponse, error) {
	return nil, locketmodels.ErrResourceNotFound
}

func (*noopLocketClient) Release(ctx context.Context, in *locketmodels.ReleaseRequest, opts ...grpc.CallOption) (*locketmodels.ReleaseResponse, error) {
	panic("not implemented")
}

func (*noopLocketClient) FetchAll(ctx context.Context, in *locketmodels.FetchAllRequest, opts ...grpc.CallOption) (*locketmodels.FetchAllResponse, error) {
	return &locketmodels.FetchAllResponse{}, nil
}
