# homekit-amt8000

This projects creates a homekit bridge and accessories for the Intelbras AMT8000
alarm system.

## Configuration

This project is set up using environment variables.

Full list of them and what they are for:

```sh
# Alarm IP addr.
# required.
HOST=192.168.207.4

# Alarm password.
# required.
PASSWORD=123456

# Cental port.
# default: 9009.
PORT=9009

# Zones that are motion sensors.
MOTION="1,2,3"

# Zones that are contact sensors (i.e. doors).
CONTACT="4,5,6"

# Indexed zone names.
# default: [Zone 1, Zone 2, ...]
ZONE_NAMES="Kitchen door,Living Room Window"

# Partition to arm when set stay mode.
# default: 1
STAY="1"

# Partition to arm when set to night mode.
# default: 2
NIGHT="2"

# Partition to arm when set to away mode.
# default: 0 (all partitions)
AWAY="0"
```

> _Warning_
> the away mode of the homekit bridge does not translate to the per-manual
> stay mode in the Intelbras alarm system, mainly because it is supper confusing.
> Instead, the alarm system here has 4 states:
>
> - Off
> - Night (which will arm the `$NIGHT` partition per configuration)
> - Away (which will arm the `$AWAY` partition per configuration)
> - Home (which will arm the `$STAY` partition per configuration)

## Running

```bash
source .env
go run .
```

## Pin

Open the Home app, add new accessory, the security system should show up.
Setup code is `001 02 003`.

## TODO

- [x] panic switch
- [x] bypass zones (?)
- [ ] show zones firing
- [ ] show partitions firing
- [ ] battery statuses
- [ ] read alarm mac addr

## License

[The "Intelbras Documentation Sucks" License](./LICENSE.md).

## Previous work and thanks

- https://github.com/elvis-epx/alarme-intelbras
- https://github.com/thspinto/isecnet-go
- Wireshark
