# homekit-amt8000

This projects creates a Homekit bridge and accessories for the Intelbras AMT8000
alarm system.

###### Security System + Panic button

![CleanShot 2023-10-22 at 23 17 51@2x](https://github.com/caarlos0/homekit-amt8000/assets/245435/9413ad39-24d7-43be-b2a5-e602453a2084)

![CleanShot 2023-10-22 at 23 18 04@2x](https://github.com/caarlos0/homekit-amt8000/assets/245435/de9222ce-0fac-4640-921a-bd55796ed311)

![CleanShot 2023-10-22 at 23 18 16@2x](https://github.com/caarlos0/homekit-amt8000/assets/245435/304b1428-ddd2-4f4a-896e-c648e3388e12)

###### Zones (motion and contact) + Bypass switches

![CleanShot 2023-10-22 at 23 17 14@2x](https://github.com/caarlos0/homekit-amt8000/assets/245435/f6ea5419-3161-4463-9557-fc02cdd96a52)

![CleanShot 2023-10-22 at 23 17 28@2x](https://github.com/caarlos0/homekit-amt8000/assets/245435/a0ef7ef4-8102-4707-a124-e9921bb8aeef)

## Configuration

This project is set up using environment variables.

Full list of them and what they are for:

```sh
# Alarm IP addr.
# required.
HOST=192.168.207.4

# Alarm password.
# Should be the remote configuration password (usually 6 digits).
# required.
PASSWORD=123456

# Cental port.
# default: 9009.
PORT=9009

# Zones that are motion sensors.
MOTION="1,2,3"

# Zones that are contact sensors (i.e. doors, windows).
CONTACT="4,5,6"

# Zones to show the bypass switch for.
BYPASS='1,2,3,4,5,6,7,8'

# Indexed zone names.
# default: "Zone 1,Zone 2,..."
ZONE_NAMES="Kitchen door,Living Room Window"

# Partitions to arm when set stay mode.
# required.
STAY="1,2"

# Partitions to arm when set to night mode.
# required.
NIGHT="2,3"

# Partitions to arm when set to away mode.
# required.
AWAY="0"

# Siren numbers you want to be shown.
# It'll show them as a contact sensor, with Tamper and Battery status.
SIRENS="1,2"

# Repeater numbers you want to be shown.
# It'll show them as a contact sensor, with Tamper and Battery status.
REPEATERS="1,2"

# If the alarm is triggered, and you turn it off, after this amount of time the
# bridge will automatically clean the firings.
# If empty or 0, it will not automatically do that.
CLEAN_FIRINGS_AFTER=5m
```

> [!WARNING]
> the away mode of the Homekit bridge does not translate to the per-manual
> stay mode in the Intelbras alarm system, mainly because it is supper confusing.
> Instead, the alarm system here has 4 states:
>
> - Off
> - Night: arms the `$NIGHT` partitions
> - Away arms the `$AWAY` partitions
> - Home arms the `$STAY` partitions
>
> Rationale for what to do in each of them can be found
> [here](https://www.commandone.com/what-is-the-difference-between-stay-away-and-night-home-alarm-activation-modes/),
> but basically you'll want to arm all partitions on **away**, and a subset of
> partitions on **night** and **stay**, depending on your use case.
>
> For me, I created 3 partitions:
>
> 1. Inside motion sensors
> 2. Outside motion sensors
> 3. Access sensors and doors
>
> And I set it up like so:
>
> ```sh
> AWAY=0
> STAY=2
> NIGHT=2,3
> ```
>
> But, you do you. 😄

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
- [x] show zones firing
- [ ] show partitions firing
- [x] battery statuses
- [x] read alarm mac addr
- [ ] receive notifications from the alarm system

## License

[The "Intelbras Documentation Sucks" License](./LICENSE.md).

> [!CAUTION]
> Be mindful of the license.
> Go, read it.

## Previous work and thanks

- https://github.com/elvis-epx/alarme-intelbras
- https://github.com/thspinto/isecnet-go
- Wireshark

## FAQ

### Can you add this thing I need?

No.

### Can you merge my PR?

Unlikely.

### Did you enjoy writing this?

Not in the slightest.
