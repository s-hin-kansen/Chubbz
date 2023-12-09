# chubbz

Simulation 1:
Normal & no faults

Simulation 2:
Intermittent down upon client requests, but comes back up soon after

Simulation 3:
Permanent down after granting FIRST lock

Simulation 4:
Permanent down AFTER receiving release & data replication but before replying RELEASE OK

Simulation 5:
"Permanent" down, comes back alive after Backup takes control

## To run the simulations

- Ensure that docker is installed locally and is running

1. Copy the contents of `docker-compose-sim<simulation number>.yaml` into `docker-compose.yaml`

2. Run `docker-compose up `
