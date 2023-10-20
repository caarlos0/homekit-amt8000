# homekit-amt8000

This projects creates a homekit bridge and accessories for the Intelbras AMT8000
alarm system.

## Configuration

This project is set up using environment variables.

Full list of them and what they are for:

```sh
# Central IP addr.
# required.
HOST=192.168.207.4

# Central password.
# required.
PASSWORD=123456

# Cental port.
# default: 9009.
PORT=9009

# Zones that are motion sensors.
MOTION="1,2,3"

# Zones that are contact sensors (i.e. doors).
CONTACT="4,5,6"

# Partition to arm when set stay mode.
# default: 1
STAY="1"

# Partition to arm when set to night mode.
# default: 2
NIGHT="2"

# Partition to arm when set to away mode.
# default: 0 (all partitions)
AWAY="0"

# Indexed zone names.
# default: [Zone 1, Zone 2, ...]
ZONE_NAMES="Kitchen door,Living Room Window"

# Zones that allow bypass.
ALLOW_BYPASS="1,2,3,4"
```

## Running

```bash
source .env
go run .
```

## Pin

Open the Home app, add new accessory, the security system should show up.
Setup code is `001 02 003`.

## TODO

- [ ] panic switch
- [x] bypass zones (?)
- [ ] show zones firing
- [ ] show partitions firing

## License

[The "Intelbras Documentation Sucks" License](./LICENSE.md).

## Previous work and thanks

- https://github.com/elvis-epx/alarme-intelbras
- https://github.com/thspinto/isecnet-go
- Wireshark
