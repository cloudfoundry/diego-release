package bbsresolver

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
)

// Resolver maps a Garden container handle to its CF process type by querying BBS.
type Resolver struct {
	bbs    bbs.InternalClient
	cellID string
	logger lager.Logger
}

// New returns a Resolver that queries client for ActualLRPs on cellID.
func New(client bbs.InternalClient, cellID string, logger lager.Logger) *Resolver {
	return &Resolver{bbs: client, cellID: cellID, logger: logger}
}

// ProcessType resolves a Garden handle (== ActualLRP.InstanceGuid) to its
// process type. It locates the matching ActualLRP on the cell, fetches the
// DesiredLRP for its ProcessGuid, and reads the process_type metric tag.
func (r *Resolver) ProcessType(ctx context.Context, handle string) (string, error) {
	lrps, err := r.bbs.ActualLRPs(r.logger, "", models.ActualLRPFilter{CellID: r.cellID})
	if err != nil {
		return "", err
	}

	var match *models.ActualLRP
	for _, lrp := range lrps {
		if lrp.InstanceGuid == handle {
			match = lrp
			break
		}
	}
	if match == nil {
		return "", fmt.Errorf("no actual lrp for handle %q on cell %q", handle, r.cellID)
	}

	desired, err := r.bbs.DesiredLRPByProcessGuid(r.logger, "", match.ProcessGuid)
	if err != nil {
		return "", err
	}

	tag := desired.GetMetricTags()["process_type"]
	if tag == nil || tag.Static == "" {
		return "", fmt.Errorf("desired lrp %q has no process_type metric tag", match.ProcessGuid)
	}

	return tag.Static, nil
}
