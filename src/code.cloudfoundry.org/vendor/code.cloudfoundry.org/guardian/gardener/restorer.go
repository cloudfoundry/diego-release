package gardener

import "code.cloudfoundry.org/lager/v3"

type restorer struct {
	networker Networker
}

func NewRestorer(networker Networker) Restorer {
	return &restorer{
		networker: networker,
	}
}

func (r *restorer) Restore(logger lager.Logger, handles []string) []string {
	failedHandles := []string{}

	for _, handle := range handles {
		log := logger.Session("looking-for-properties", lager.Data{"handle": handle})

		err := r.networker.Restore(logger, handle)
		if err != nil {
			log.Error("failed-restoring-container", err)
			failedHandles = append(failedHandles, handle)
		}
	}

	return failedHandles
}
