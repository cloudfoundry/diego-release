package models

import (
	"google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

//go:generate bash ../scripts/generate_protos.sh
//go:generate counterfeiter . LocketClient

const PresenceType = "presence"
const LockType = "lock"

var ErrLockCollision = status.Errorf(codes.AlreadyExists, "lock-collision")
var ErrInvalidTTL = status.Errorf(codes.InvalidArgument, "invalid-ttl")
var ErrInvalidOwner = status.Errorf(codes.InvalidArgument, "invalid-owner")
var ErrResourceNotFound = status.Errorf(codes.NotFound, "resource-not-found")
var ErrInvalidType = status.Errorf(codes.NotFound, "invalid-type")
