---
title: The Simulation
expires_at : never
tags: [diego-release, auction]
---

# The Simulation

The simulation spins up a number of in-process
[SimulationReps](https://github.com/cloudfoundry/auction/blob/master/simulation/simulationrep/simulation_rep.go).
They implement the [Rep client
interface](https://github.com/cloudfoundry-incubator/rep/blob/master/client.go#L41-L54).
This in-process communication mode allows us to isolate the algorithmic details
from the communication details. It allows us to iterate on the scoring math and
scheduling details quickly and efficiently.
