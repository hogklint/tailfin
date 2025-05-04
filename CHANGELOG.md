# v0.2.0

## :warning: Breaking Changes

The templating variable names are updated to better reflect reality and to prepare for future Swarm support.

* `ContainerName` is now the container name, and not the service name when using Docker Compose
* `ServiceName` is added to give the Docker Compose service name
  * For convenience, it's the container name for non-Compose usage
* `ComposeProject` is renamed `Namespace` to use the same variable for Swarm namespace/stack in the future
* `ContainerNumber` is added to give the container instance number

As such, the color function `ComposeColor` is renamed `NamespaceColor`, and the flag `--compose-color` is renamed
`--namespace-color`.

## Changes
* Trim carriage return from TTY log lines

# v0.1.1

## Changes
* Updated json output to less verbose names
* Improve resuming logs on container restart
  * Using the container stop time sometimes includes already printed logs

# v0.1.0

## Changes
* Add `--compose` flag
* Add `--label` flag
* Update how `--since` behaves; including logs older than latest container start
* Fix tailing hanging on absent docker daemon

# v0.0.2

## Changes
Fix panic when trimming log lines with invalid format

# v0.0.1

## Changes
First release of tailfin
