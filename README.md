# homekit-amt8000

## Configuration:

```sh
HOST=192.168.207.4     # mandatory
PASSWORD=123456        # mandatory
PORT=9009              # default is 9009
MOTION="1,2,3"         # zones that are motion sensors
CONTACT="4,5,6"        # zones that are contact sensors (i.e. doors)
STAY="1"               # partition to arm when set stay mode - default is 1
NIGHT="2"              # partition to arm when set to night mode - default is 2
AWAY="255"             # partition to arm when set to away mode - default is 255
```

## Running

```bash
source .env
go run .
```

### Partitions

- `0xff` (255) means all partitions
- `0x01` (1) means partition 1
- you get the idea

Open the Home app, add new accessory, the security system should show up.
Setup code is `001 02 003`.

## TODO

- [ ] panic switch
- [x] bypass zones (?)
- [ ] show zones firing
- [ ] show partitions firing

## License

[The "Intelbras Documentation Sucks" License](./LICENSE.md).

## Previous work

- https://github.com/elvis-epx/alarme-intelbras
- https://github.com/thspinto/isecnet-go
