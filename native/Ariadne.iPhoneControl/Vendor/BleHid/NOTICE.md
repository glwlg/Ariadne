# BleHid core

Source: https://github.com/abhishek-raj/windows-ble-hid
Revision: b33a0043b3865ea8251a242eae27aa2f72816512
License: MIT (see LICENSE).

Only the HID peripheral and its dependencies are included. Ariadne does not use
global input hooks, startup registration, or input broadcast.

Local changes: store pacing configuration below Ariadne's airplay data directory;
ignore cancelled GATT requests; expose mouse subscriptions; validate notification
results so a failed transmission is reported; select hosts by stable device ID
and disable broadcast notifications.

Direct video input adds absolute mouse report ID 3 (0..32767 axes) alongside the
existing relative mouse report ID 2. The absolute-pointer / relative-button
combination and report layout follow iPhone Use's iOS HID implementation:
https://github.com/xhoantran/iphone-use
Revision: bc0fe36e78ec5a3af095c813574e3a8b7bbea4dd
Copyright (c) 2026 NewWave AI, MIT (see LICENSE-iPhoneUse).
Window messaging, coordinate mapping and input scheduling are implemented locally.
Constant HID information/report-map values use WinRT StaticValue so the Windows
stack handles offset reads when the report map exceeds the negotiated ATT payload.
