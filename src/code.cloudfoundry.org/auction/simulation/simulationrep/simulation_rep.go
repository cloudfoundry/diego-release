package simulationrep

import (
	"net/http"
	"sync"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep"
)

type SimulationRep struct {
	cellID                 string
	cellIndex              int
	stack                  string
	zone                   string
	totalResources         rep.Resources
	lrps                   map[string]rep.LRP
	tasks                  map[string]rep.Task
	startingContainerCount int
	volumeDrivers          []string

	lock *sync.Mutex
}

func New(cellID string, cellIndex int, stack string, zone string, totalResources rep.Resources, volumeDrivers []string) rep.SimClient {
	return &SimulationRep{
		cellID:                 cellID,
		cellIndex:              cellIndex,
		stack:                  stack,
		totalResources:         totalResources,
		lrps:                   map[string]rep.LRP{},
		tasks:                  map[string]rep.Task{},
		startingContainerCount: 0,
		zone:                   zone,
		volumeDrivers:          volumeDrivers,

		lock: &sync.Mutex{},
	}
}

func (r *SimulationRep) State(_ lager.Logger) (rep.CellState, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	lrps := []rep.LRP{}
	for _, lrp := range r.lrps {
		lrps = append(lrps, lrp)
	}

	tasks := []rep.Task{}
	for _, task := range r.tasks {
		tasks = append(tasks, task)
	}

	availableResources := r.availableResources()

	// util.RandomSleep(800, 900)

	return rep.CellState{
		CellID:    r.cellID,
		CellIndex: r.cellIndex,
		RootFSProviders: rep.RootFSProviders{
			models.PreloadedRootFSScheme: rep.NewFixedSetRootFSProvider(r.stack),
		},
		AvailableResources:     availableResources,
		TotalResources:         r.totalResources,
		LRPs:                   lrps,
		Tasks:                  tasks,
		StartingContainerCount: r.startingContainerCount,
		Zone:                   r.zone,
		VolumeDrivers:          r.volumeDrivers,
	}, nil
}

func (r *SimulationRep) Perform(_ lager.Logger, work rep.Work) (rep.Work, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	failedWork := rep.Work{}

	availableResources := r.availableResources()

	for _, start := range work.LRPs {
		hasRoom := availableResources.Containers >= 0
		hasRoom = hasRoom && availableResources.MemoryMB >= start.MemoryMB
		hasRoom = hasRoom && availableResources.DiskMB >= start.DiskMB

		if hasRoom {
			r.lrps[start.Identifier()] = start

			availableResources.Containers -= 1
			if start.Domain == "auction" {
				r.startingContainerCount++
			}
			availableResources.MemoryMB -= start.MemoryMB
			availableResources.DiskMB -= start.DiskMB
		} else {
			failedWork.LRPs = append(failedWork.LRPs, start)
		}
	}

	for _, task := range work.Tasks {
		hasRoom := availableResources.Containers >= 0
		hasRoom = hasRoom && availableResources.MemoryMB >= task.MemoryMB
		hasRoom = hasRoom && availableResources.DiskMB >= task.DiskMB

		if hasRoom {
			r.tasks[task.TaskGuid] = task

			availableResources.Containers -= 1
			if task.Domain == "auction" {
				r.startingContainerCount++
			}
			availableResources.MemoryMB -= task.MemoryMB
			availableResources.DiskMB -= task.DiskMB
		} else {
			failedWork.Tasks = append(failedWork.Tasks, task)
		}
	}

	return failedWork, nil
}

//simulation only

func (r *SimulationRep) Reset() error {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.lrps = map[string]rep.LRP{}
	r.tasks = map[string]rep.Task{}
	r.startingContainerCount = 0
	return nil
}

//these are rep client methods the auction does not use

func (rep *SimulationRep) StopLRPInstance(lager.Logger, models.ActualLRPKey, models.ActualLRPInstanceKey) error {
	panic("UNIMPLEMENTED METHOD")
}

func (rep *SimulationRep) UpdateLRPInstance(lager.Logger, rep.LRPUpdate) error {
	panic("UNIMPLEMENTED METHOD")
}

func (rep *SimulationRep) CancelTask(lager.Logger, string) error {
	panic("UNIMPLEMENTED METHOD")
}

func (rep *SimulationRep) SetStateClient(client *http.Client) {
	panic("UNIMPLEMENTED METHOD")
}

func (rep *SimulationRep) StateClientTimeout() time.Duration {
	panic("UNIMPLEMENTED METHOD")
}

//internal -- no locks here

func (rep *SimulationRep) availableResources() rep.Resources {
	resources := rep.totalResources
	for _, lrp := range rep.lrps {
		resources.MemoryMB -= lrp.MemoryMB
		resources.DiskMB -= lrp.DiskMB
		resources.Containers -= 1
	}
	for _, task := range rep.tasks {
		resources.MemoryMB -= task.MemoryMB
		resources.DiskMB -= task.DiskMB
		resources.Containers -= 1
	}
	return resources
}
