# Air780EPV GSM7 PDU fix

This directory contains the matched core and Lua script for the Air780EPV only.
Do not flash these files to an Air780EHV or another module model.

## Files

- `LuatOS-SoC_V1002_EC718PV_SMS-PDU-20260721.soc`: V1002 EC718PV core with the original received PDU exposed to Lua.
- `main.lua`: plugin 1.4.0; sends each PDU segment to the server and disables the V1002 long-SMS merge path.
- `luat_lib_sms_pdu_metadata.patch`: reproducible LuatOS core source change used by the custom `.soc`.

SHA256:

```text
09B8E46B485BC4A7B210958B7F7FDF7574ACE0D2B35C7647B6293E532F65DF21  LuatOS-SoC_V1002_EC718PV_SMS-PDU-20260721.soc
BA716B4FC172C3C94FF9F40E673BC8D0843578B5273911E77D4426159D67F055  main.lua
```

## Flash once

1. Upgrade the server platform first. The old platform does not decode the new `pdu_hex` field.
2. In LuaTools select this `.soc` as the core firmware.
3. Select this directory as the script project so `main.lua` is included in the same download operation.
4. Download once, then confirm the module status reports plugin version `1.4.0`.

The core build and fixed PDU regression vectors are verified. A real giffgaff
GSM7 message is still required for final hardware acceptance.
