# homekit-amt8000

This projects creates a homekit bridge and accessories for the Intelbras AMT8000
alarm system.

###### Security System + Panic button

![CleanShot 2023-10-22 at 20 31 56@2x](https://github.com/caarlos0/homekit-amt8000/assets/245435/57fe0b21-9020-4208-97e9-adbbd3e9170f)

![CleanShot 2023-10-22 at 20 32 35@2x](https://github.com/caarlos0/homekit-amt8000/assets/245435/b08d928d-1e9d-4d99-a0be-33483732240a)

###### Zones (motion and contact) + Bypass switches

![CleanShot 2023-10-22 at 20 38 30@2x](https://github.com/caarlos0/homekit-amt8000/assets/245435/67edc61d-f0a6-4b13-91fc-9cfc3fcc63da)

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

# Partitions to arm when set stay mode.
# default: 1
STAY="1,2"

# Partitions to arm when set to night mode.
# default: 2
NIGHT="2,3"

# Partitions to arm when set to away mode.
# default: 0 (all partitions)
AWAY="0"
```

> **Warning**
> the away mode of the homekit bridge does not translate to the per-manual
> stay mode in the Intelbras alarm system, mainly because it is supper confusing.
> Instead, the alarm system here has 4 states:
>
> - Off
> - Night: arms the `$NIGHT` partitions
> - Away arms the `$AWAY` partitions
> - Home arms the `$STAY` partitions
>
> Rationale for what to do in each of them can be found
> [here](https://www.commandone.com/what-is-the-difference-between-stay-away-and-night-home-alarm-activation-modes/).

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
- [x] multiple partitions per state
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
