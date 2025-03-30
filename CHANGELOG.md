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
