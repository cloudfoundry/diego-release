# RunDMC - Diminuatively Managed RunC Containers

RunDMC is a small wrapper around runC. 


## High Level Architecture

Each container is stored as a subdirectory of a directory called 'the depot'. 
The depot is the source of truth for RunDMC, when a container is created, this amounts
to creating an Open Container Spec compliant container as a subdirectory of the depot directory.
The subdirectory is named after the container's handle. Looking up a container amounts to
checking for the presence of a subdirectory with the right name. 

To execute processes in a container, we launch the runc binary inside the container directory
and pass it a custom process spec. Since we want to control the container lifecycle via the API without
the restriction that the container dies when its first process dies, the containers are always
created with a no-op initial process that never exits. User processes are all executed using `runc exec`.

The process_tracker allows reattaching to running containers when RunDMC is restarted. It holds on to
process input/output streams and allows reconnecting to them later.

