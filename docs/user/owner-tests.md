# Owner test plan / Owner test planı (VLT-IS-004 TASK 03-00-00-009, VLT-IS-005 TASK 03-00-00-009, VLT-IS-006 TASK 03-00-00-009, VLT-IS-007 TASK 03-00-00-009, VLT-IS-008 TASK 03-00-00-007, VLT-IS-009 TASK 03-00-00-007, VLT-IS-010 TASK 03-00-00-007)

English first. Turkish: section T below.

## E. English

### Preparation

1. Open the Releases page of the repository: https://github.com/abuzer1495/voltiovolt/releases
2. Open the pre-release "Test build" (the tag is test-latest).
3. Download VoltioVolt-test-windows-amd64.zip or VoltioVolt-test-linux-amd64.tar.gz.
4. Check the SHA-256 values in SHA256SUMS.txt (docs/user/probe.md section 1).
5. Stop UPSilon 2000 and the RupsMon service before you start VoltioVolt. Start them again after the tests if you need them.
6. Windows: extract the zip and start VoltioVolt.exe from the extracted folder. No administrator rights are necessary. The Microsoft Edge WebView2 Runtime is necessary (Windows 10 and 11 have it). Method 2: run install.cmd (it copies the files to %LOCALAPPDATA%\Programs\VoltioVolt and asks before it makes a Start menu shortcut).
7. Linux: extract the tar.gz and start ./voltiovolt. Run-time packages: libgtk-4-1 and libwebkitgtk-6.0-4 (for example on Debian and Ubuntu: `sudo apt install libgtk-4-1 libwebkitgtk-6.0-4`). Method 2: run install.sh (it copies the files to ~/.local/opt/voltiovolt and asks before it makes a desktop shortcut). Do not use sudo for the install script.
8. If the VLT-IS-007 test was installed: run remove-old-test-service.cmd one time (this script asks for administrator rights).

Each test has: purpose, preconditions, steps, expected result, and what to send to the Instruction Author. Do the tests in order. Send the results of each test as soon as it is complete. Do not send a Secret, a PIN, a DEK, a QID value or a serial number. Check each file before you send it.

General preconditions for all tests:

- A test computer, not a production computer. Save all work before each test.
- The Reference UPS and its USB or serial cable.
- The test package of the latest pre-release "Test build" (tag test-latest). See "Preparation" above.
- PIN is optional in portable mode. If no PIN is set, all functions are available. Secret fields stay write-only.
- Do the tests in this order: T-01 first, then T-08 to T-17. The tests T-02, T-03, T-05, T-07 and the service parts of T-04 and T-06 are in the section "Service mode (later)".

### T-01 vlt-probe Capture on the Reference UPS

- Purpose: find the Dialect and the real responses of the Reference UPS (VLT-IS-002 TASK 03-00-00-008).
- Preconditions: vlt-probe from the extracted test package (Windows: vlt-probe.exe, Linux: vlt-probe). The UPS is connected with USB. Use Linux if possible, because Windows has no raw report descriptor. The UPS battery is fully charged.
- Preconditions (VLT-IS-006):
    - Stop UPSilon 2000 and the RupsMon service before the test. Start them again after the test.
    - The Reference UPS uses serial Megatec (Q1, F, I) on a virtual COM port (OD-26). Use "--transport serial --port COM3" on Windows. On Linux, find the port with "vlt-probe scan" (/dev/ttyUSB* or /dev/ttyACM*).
    - Write the hardware ID of the port (Windows Device Manager: Ports, COM3, Properties, Details, Hardware Ids).
- Steps:
    1. Follow docs/user/probe.md: `vlt-probe scan`, then `vlt-probe probe --transport hid` (Linux: with sudo).
    2. Do the mains test of docs/user/probe.md section 4 (watch mode, 3 minutes, input power off for 60 seconds or less). Write down the two times.
    3. If the UPS has a serial port: do step 1 again with `--transport serial --port <port>`.
- Expected result: one Capture file for each run. The probe Capture has a response for Q1 or QS. The watch Capture shows status bit 7 = 1 while the input power is off.
- Send: the Capture files, the two times, the OS name and version, and the USB vendor and product IDs from `scan`.

### T-08 Policy Simulation

- Purpose: check the Simulation of the default policy and of a changed policy (VLT-IS-004 TASK 03-00-00-006) with VoltioVolt. Also check the Simulation step "OS shutdown" (VLT-IS-008 TASK 03-00-00-004).
- Preconditions: The test package is prepared as in "Preparation". No UPS is necessary. Do not connect a UPS for this test: the Simulation does not turn off the computer, but an active policy can turn it off when a UPS reports a power failure. No group membership and no administrator rights are necessary.
- Steps:
    1. Start VoltioVolt from the extracted folder (Windows: VoltioVolt.exe, Linux: ./voltiovolt). If the first-run wizard starts, the PIN step is optional: it has a "Skip" button. The text is: "A PIN prevents other people at this computer from changing the shutdown settings." You can click "Skip", or set a PIN. If a PIN is set, unlock with the PIN. If no PIN is set, all functions are available.
    2. Open "Shutdown Policy". Write down the version and the state (active or inactive). If the screen shows the error "This user cannot shut down the computer.", write it down. You cannot activate the policy in that case.
    3. Click "Run the Simulation". Write down the result of each scenario (passed or failed, and the time of the shutdown request or the cancel). Write down the result of the step "OS shutdown" (pass, or the same error as in step 2).
    4. If the Simulation passed (including "OS shutdown"), click "Activate". Read the confirmation text, then confirm. The state is "active".
    5. Click "Edit". Change "countdown_seconds" to 90. Click "Save".
    6. Check the text "The policy is now inactive. Run the Simulation to activate it." and the state "inactive". "Activate" is off.
    7. Click "Run the Simulation" again. Then "Activate" is on. Do not activate. Click "Edit", set "countdown_seconds" to 60 again and click "Save".
- Expected result: step 3: all scenarios "passed", and the step "OS shutdown" passed. Default policy: power_fail_long gives a shutdown request at 370 s, power_fail_short gives a cancel, low_battery gives a refused cancel and a request at 90 s, comm_lost_on_battery gives a request before its limit. Step 6: the policy is inactive after the change. Step 7: "Activate" is on only after the new Simulation passed. If "OS shutdown" failed, the error is exactly "This user cannot shut down the computer." and you cannot activate the policy.
- Send: the results of steps 2, 3, 6 and 7 (screenshots are permitted; check that no Secret is in them), the version of VoltioVolt (General, About) and the OS.

### T-09 Real shutdown with the Reference UPS

WARNING: This test turns off the computer. Save all work. Close all applications. Make sure that the UPS battery is fully charged. Set trigger.on_battery_seconds to 30. Disconnect the UPS input power. Connect the input power again after the computer is off.

- Purpose: check the full chain on real hardware: UPS, Device Manager, Event Engine, Policy Engine, OS shutdown executor (TASK 03-00-00-006 and 007).
- Preconditions: T-01 and T-08 are complete. The computer is powered by the Reference UPS, and the UPS is connected to the computer with USB or serial. VoltioVolt runs (started from the extracted folder, as in T-08). No other person uses the computer.
- Steps (all in VoltioVolt):
    1. If a PIN is set, unlock with the PIN. Open "Shutdown Policy", click "Edit", set "trigger.on_battery_seconds" to 30 (keep the other defaults: countdown 60 s, action shutdown) and click "Save".
    2. Click "Run the Simulation". The result must be "passed", including the step "OS shutdown". Then click "Activate" and confirm.
    3. Open "Device": the state must be "connected". If Linux shows "port permission denied" or "HID permission denied", follow the help text (run the command one time, then log out and log in). VoltioVolt does not run that command and does not install a udev rule.
    4. Open "Dashboard". The mode is "Mains power". Write down the time.
    5. Disconnect the UPS input power. Write down the time.
    6. Watch the Dashboard: the mode changes to "On battery", the Event ON_BATTERY is at the top of the Events, after 30 seconds the countdown banner starts with 60 seconds.
    7. The computer turns off. Write down the time.
    8. Connect the UPS input power again. Start the computer.
    9. Start VoltioVolt again. Windows: open the log file in the data folder (General, "Data folder"): logs\vltd.log, and Event Viewer, System log, source User32, event ID 1074. Linux: open "Security" and read the audit log; if you started VoltioVolt from a terminal, also keep the standard error output. Do not use `journalctl -u vltd`.
    10. If a PIN is set, unlock. Open "Security" and read the audit log (policy_countdown_start, policy_shutdown_request, os_shutdown_request). Then set "trigger.on_battery_seconds" back to 300 on "Shutdown Policy".
- Expected result: the computer turns off about 90 seconds after step 5, without a dialog. The audit log has the three entries. Windows: event 1074 shows the reason code 0x6000c ("Power Failure: Environment"; the exact event text is UNVERIFIED).
- Abort: to stop the countdown, connect the input power again before the end (cancel_when_mains_returns is on), or click "Cancel" in the countdown banner (not possible with LOW_BATTERY).
- Send: the times of steps 4, 5 and 7, the log lines, the audit entries (no Secret is in them), and the Windows event 1074 text.

### T-10 VLT GUI test

- Purpose: check the VLT GUI in VoltioVolt (VLT-IS-005) on the Owner computer, and review all Turkish text (VLT-IS-005 TASK 03-00-00-008 step 1.4). Also check the portable functions of VLT-IS-008: the data folder, start at logon, the tray, and the optional PIN.
- Preconditions: T-01 and T-08 are complete (OD-23, portable order). VoltioVolt is from the test package with the SHA-256 check. For a new first start: a new data folder, with or without a PIN (optional).
- Steps:
    1. Start VoltioVolt from the extracted folder. The engine and the VLT GUI run in one process. Close the window: the window hides and the monitoring continues. The first time, the text is: "VoltioVolt continues to monitor the UPS in the tray." Open the window from the tray. Then use the tray menu "Quit": the confirmation is "If you quit, VoltioVolt stops the monitoring and the automatic shutdown." Confirm. The application stops. Start it again.
    2. Set PIN (optional): on a new first start, the wizard PIN step has "Skip" and the text "A PIN prevents other people at this computer from changing the shutdown settings." Try a PIN with 5 characters (refused), then set a correct PIN, or click "Skip". The General screen has "Set PIN" when no PIN is set, and "Remove PIN" when a PIN is set (Remove needs the current PIN).
    3. Login (only if a PIN is set): click "Unlock" and enter a wrong PIN until the GUI shows the block time. Wait, then unlock with the correct PIN.
    4. Lock (only if a PIN is set): click "Lock". The GUI shows "Locked" and still shows the Dashboard. Unlock again. Then wait for the idle time (auth.session_idle_minutes) without a click: the GUI locks itself.
    5. Dashboard values: compare the values (input and output voltage, frequency, load, battery charge and voltage, runtime, temperature) with the UPS display or the UPS documentation.
    6. Device screen: the state "connected", the Transport, the Dialect, the model data. The serial number is not shown in a report.
    7. History charts: open each range (1 hour to 1 year) and each chart. Open the table view of one chart.
    8. Events: load older Events, filter by severity and by type.
    9. Policy edit: change one value outside its range (for example countdown_seconds 5): the GUI refuses it before it sends it. Change it to a correct value and save.
    10. Simulation: run it and check that each scenario shows its steps (including "OS shutdown"), its virtual times and "passed" or "failed".
    11. Activation: "Activate" is on only after a passed Simulation (including "OS shutdown"). Activate, then deactivate with the confirmation.
    12. Language and General: open "General". The screen shows the data folder path. The switch "Start at logon" is off by default. Turn it on, then off: VoltioVolt uses only the startup folder of the user. Select "Türkçe", then "English". Close and start VoltioVolt again: the language stays.
    13. Review of all Turkish text: select "Türkçe" and open each screen and each message that you can see. Check the Turkish characters (ç, ğ, ı, İ, ö, ş, ü), the words and the grammar.
    14. Keyboard: use only the Tab, Shift+Tab, Enter and Space keys on one screen. The focus is always visible.
    15. Theme: change the OS theme between light and dark. The GUI follows it.
- Expected result: all steps work as written. No text in the GUI is in the wrong language or has wrong Turkish characters.
- Send: the result of each step (OK or the problem), a list of the Turkish text that must change (the screen, the current text and the correct text), the OS, the data folder path, and the version of VoltioVolt (General, About). Screenshots are permitted; check that no Secret, no PIN and no serial number is in them.

### T-11 UPS Control (VLT-IS-006)

- Purpose: check the UPS Control screen and the CONTROL commands test and beeper on the Reference UPS (VLT-IS-006 TASK 03-00-00-005, 007 and 008).
- Preconditions: T-01, T-08 and T-10 are complete. VoltioVolt runs with `device.transport` serial and the port of T-01 (screen "Device"). The device state is "connected". Mains power is present. The UPS battery is fully charged.
- Steps (all in VoltioVolt):
    1. If a PIN is set, unlock with the PIN. Open "UPS Control". The buttons are on.
    2. Click "Beeper off". Write down the time. Disconnect the UPS input power for 10 seconds: the UPS makes no sound. Connect the input power again.
    3. Click "Beeper on". Disconnect the UPS input power for 10 seconds: the UPS makes a sound. Connect the input power again.
    4. Click "Test 10 seconds", then confirm with "Start the test". Write down the time. The Event TEST_STARTED shows. After the test, the screen shows the last test result (TEST_PASSED or TEST_FAILED) with its time.
    5. Open "Events": TEST_STARTED and its clear.
    6. Open "Security" and read the audit log: ups_beeper_set, ups_control, ups_control_verify, ups_test_start and ups_test_end.
- Expected result: each command is "verified". The test result is TEST_PASSED. The audit log has the entries of step 6 with the bytes (Q, T).
- Send: the times, the test result, the audit entries (no Secret is in them) and the Events.

### T-12 UPS output off and restore (VLT-IS-006)

WARNING: The UPS turns off its output during this test. All equipment on the UPS loses power. Connect only the test computer to the UPS. Save all work. Make sure that the battery is fully charged.

- Purpose: check the UPS output off step of the Shutdown Policy (VLT-IS-006 TASK 03-00-00-006, OD-28).
- Preconditions: T-09 and T-11 are complete. Only the test computer is connected to the UPS output. VoltioVolt runs with the port of T-01.
- Steps (all in VoltioVolt):
    1. If a PIN is set, unlock with the PIN. Open "Shutdown Policy", click "Edit", set ups_off.enabled on, ups_off.delay_minutes 2, ups_off.restore_minutes 1 (keep trigger.on_battery_seconds 30 of T-09) and click "Save".
    2. Run the Simulation. The result must be "passed" (including "OS shutdown") and each scenario shows the step "UPS output off command (dry run, not sent)". Activate the policy.
    3. Disconnect the UPS input power. Write down the time.
    4. Record the times of the OS shutdown, the UPS output off and the UPS output on.
    5. Run 1: connect the input power again before the restore time (before the UPS output is on again).
    6. Run 2: do steps 3 and 4 again, and do not connect the input power. Write down what the UPS does. Then connect the input power.
    7. Start the computer. Start VoltioVolt. Open "Security" and read the audit log: policy_countdown_start, policy_ups_off, policy_shutdown_request, os_shutdown_request.
    8. Set ups_off.enabled off again (the default, OD-28), run the Simulation and activate the policy.
- Expected result: the OS shuts down before the UPS output turns off (2 minutes after the shutdown starts). Run 1: the UPS output turns on 1 minute after it turned off. Run 2: the behavior without input power is UNVERIFIED (VLT-REF-001 5.1 item 4); write it down.
- Send: the times of steps 3 and 4 for both runs, the audit entries and the log lines (Windows: the log file in the data folder, logs\vltd.log).

### T-13 Notifications (VLT-IS-007)

- Purpose: check the desktop, Telegram and e-mail Channels (VLT-IS-007 TASK 03-00-00-004, 005 and 006).
- Preconditions: T-09 is complete. VoltioVolt from the test package is running. A Telegram bot token and a Gmail app password (the Owner's). Do not send the token or the password.
- Steps (all in VoltioVolt, screen "Notifications", unless a step says otherwise):
    1. Desktop: start VoltioVolt. Disconnect the UPS input power. A desktop notification appears. Connect the input power. Write down the time.
    2. Telegram: enter the bot token (write-only). Click "Pair a chat". Send `/start <code>` to the bot from Telegram. Send `/durum`. Disconnect the UPS input power. A Telegram Notification arrives. Connect the input power. Write down the time.
    3. E-mail: select the preset "Gmail". Enter the Gmail address and the app password (help text: "Use a Google app password, not your normal password. Google account, Security, 2-Step Verification, App passwords."). Click "Send test message". Disconnect the UPS input power. An e-mail Notification arrives. Connect the input power. Write down the time.
- Expected result: a desktop notification, a Telegram message (including a reply to `/durum` with the UPS status) and an e-mail, each during the power failure. The test e-mail arrives.
- Send: the times and a confirmation that each Channel worked. Do not send the token, the password or the message text (it can contain status data).


### T-14 PIN set, lock, unlock, restart and Forgot PIN (VLT-IS-009)

- Purpose: check that a correct PIN unlocks the GUI (including after a restart), and check "Forgot PIN" (VLT-IS-009 TASK 03-00-00-001 and 002, OD-40, R-02).
- Preconditions: VoltioVolt from the test package is running in portable mode. Use a data folder where you can set a PIN. Do not send the PIN.
- Steps (all in VoltioVolt):
    1. Set PIN: if no PIN is set, open "General" and click "Set PIN", or use the PIN step of the setup wizard. Set a PIN of at least 6 characters. If a PIN is already set from an earlier test, you can keep it.
    2. Lock: click "Lock". The GUI shows "Locked" and still shows the Dashboard.
    3. Unlock: click "Unlock", enter the correct PIN, and click "Unlock" again (the button on the login screen). The GUI unlocks. This is the function of R-02.
    4. Restart: use the tray menu "Quit", confirm, and start VoltioVolt again from the same folder and the same data folder.
    5. Unlock after the restart: enter the same correct PIN and click "Unlock". The GUI unlocks.
    6. Forgot PIN: click "Lock", then click "Forgot PIN". The text is: "This removes the PIN and deletes all saved passwords and tokens (Telegram, e-mail). The automatic shutdown becomes inactive. Your settings and your history stay." The confirmation is: "Type RESET to confirm." Type RESET (this word is the same in English and in Turkish). Click "Reset PIN".
    7. After the reset: the setup wizard starts at the PIN step. The Shutdown Policy is inactive. Settings, Events and History stay. Saved passwords and tokens are deleted. Open "Security" and find the audit entry of the reset.
- Expected result: steps 3 and 5 unlock with the correct PIN. Step 6 removes the PIN, deletes the Secrets and makes the policy inactive. Settings and history stay. The confirmation word is RESET.
- Owner sequence after the R-08 fix (VLT-IS-011 TASK 03-00-00-001, OD-50, OD-51, OD-52). Do these steps first, with a new data folder:
    1.1. Start VoltioVolt with a new data folder. The setup wizard starts at the PIN step.
    1.2. Set a PIN of at least 6 characters and click "Set PIN".
    1.3. The wizard opens the step "UPS port". VoltioVolt does not show "Locked" and does not ask for the PIN.
    1.4. Click "Test connection (simulator)". The setting is saved and the step shows the selected port. VoltioVolt does not ask for the PIN.
    1.5. Click "Next" to the Shutdown Policy step. Click "Run the Simulation", then "Activate", and confirm. VoltioVolt does not ask for the PIN.
    1.6. Go to the end of the wizard and click "Finish". The Dashboard opens and VoltioVolt is unlocked.
    1.7. Click "Lock": the lock screen opens. Enter the PIN and click "Unlock": VoltioVolt is unlocked.
    Repeat steps 1.3 to 1.7 with the PIN set in the General screen ("Set PIN") instead of the wizard: after "Set PIN", VoltioVolt stays unlocked. The lock screen never covers the wizard: if a wizard step needs the PIN, the PIN field is inside the wizard dialog. VoltioVolt locks after 10 minutes without keyboard or mouse input; the live values do not keep it unlocked.
- Send: the result of each step (OK or the problem) and the audit entry. Do not send the PIN. Screenshots are permitted; check that no PIN and no Secret is in them.

### T-15 Wizard and screens at 1280 × 720 (VLT-IS-009)

- Purpose: check the first-start dialog, the compact wizard and all screens at 1280 × 720 and at the display scale of the Owner (VLT-IS-009 TASK 03-00-00-003, R-04, OD-41).
- Preconditions: VoltioVolt from the test package. For the first-start dialog, use a new data folder (for example a new empty folder, started with the data-folder option or a fresh extract). Write down the display scale of the Owner computer (Windows: Settings, System, Display, Scale; for example 100 %, 125 % or 150 %).
- Steps:
    1. First start: the dialog is "Start the setup wizard?" with "Start" and "Later". Click "Later": the Dashboard opens. Quit and start VoltioVolt again: the dialog shows again. Click "Later" a second time: the dialog does not show again. The General screen still has the button "Start the setup wizard".
    2. Wizard size: with a new data folder, or from General, click "Start" or "Start the setup wizard". The wizard is a dialog of 760 × 560 pixels. The buttons Back, Skip and Next (or Finish) are at the bottom right. Go through each step (PIN, UPS port, live values, Shutdown Policy, Telegram, e-mail, modem and router, Summary). At 1280 × 720, each step fits in the dialog without a scroll bar. The help lines and the example values of the fields are visible.
    3. Screens at 1280 × 720: after the wizard (Finish or Later), open each screen at 1280 × 720: Dashboard, Device, History, Events, Shutdown Policy, UPS Control, Notifications, Security, General. Each screen is usable: no important control is cut off.
    4. Display scale of the Owner: keep the usual scale of the Owner computer. Repeat step 3 (and step 2 if you can). Write down the scale.
- Expected result: the first-start dialog works as written in step 1. Each wizard step fits in 760 × 560 pixels without a scroll bar. The screens work at 1280 × 720 and at the display scale of the Owner.
- Send: the result of each step, the display scale, and the OS. Screenshots are permitted; check that no Secret, no PIN and no serial number is in them. If a step or a screen has a scroll bar or a cut-off control, write it down and send a screenshot of that step or screen.

### T-16 Demo mode (VLT-IS-009)

- Purpose: check the Demo (simulator) transport (VLT-IS-009 TASK 03-00-00-005, OD-43). This test does not use a real UPS. It must not shut down the computer and must not send CONTROL bytes to a real UPS.
- Preconditions: VoltioVolt from the test package is running. If a PIN is set, unlock with the PIN. A real UPS is not necessary.
- Steps (all in VoltioVolt):
    1. Open "Device" (or the UPS port step of the wizard). Select the transport "Demo (simulator)". A yellow banner "DEMO" is on all screens.
    2. Open the Dashboard: the values change slowly. Open "Events" and "History": the demo Events and measurements are hidden by default. The checkboxes "Show other sources" are off. Turn them on: the demo data is visible and marked "demo".
    3. On the Device screen, click "Simulate power failure". The simulator goes to battery for 60 seconds, then the mains power returns.
    4. If the Shutdown Policy would shut down the computer, the message is: "Demo: the computer would shut down now." The computer does not shut down.
    5. Open "UPS Control" and start a short test or change the beeper. The command is a dry run: no CONTROL bytes go to a real UPS, and the computer does not change.
    6. If a Notification Channel is configured, a notification has the prefix "[DEMO] ". Do not send a Secret.
    7. Leave demo mode: select a different transport or a real port. The confirmation is: "Leave demo mode and connect to a real UPS? Events and measurements of the demo stay, marked as demo." Confirm or cancel as you wish. The demo history stays, marked as demo.
- Expected result: the yellow DEMO banner is visible. The values change slowly. "Simulate power failure" lasts 60 seconds. The computer does not shut down. No CONTROL bytes go to a real UPS. Demo Events and measurements are hidden by default and marked "demo". The leave confirmation is the exact text of step 7.
- Send: the result of each step (OK or the problem). Do not send a Secret. Screenshots are permitted; check that no Secret, no PIN and no serial number is in them.

### T-17 Simulator test (VLT-IS-010)

- Purpose: check the UPS simulator and the test connection (VLT-IS-010, RQ-01, RQ-02, RQ-03). This test does not use a real UPS. It must not shut down the computer (leave `test.real_shutdown` off) and must not send CONTROL bytes to a real UPS.
- Preconditions: the test package is extracted. vlt-simulator is in the same folder as VoltioVolt (`vlt-simulator.exe` on Windows, `./vlt-simulator` on Linux). A real UPS is not necessary. If a PIN is set, unlock with the PIN. Leave the Setting "Permit a real computer shutdown in a test" off. See docs/user/simulator.md for the screen, the keys, the menu and the scenarios.
- Steps:
    1. Start the simulator (Windows: `vlt-simulator.exe` in a cmd window; Linux: `./vlt-simulator` in a terminal). The screen "VoltioVolt UPS Simulator" opens. The state is "waiting".
    2. Start VoltioVolt. Open Device and select "Test connection (simulator)". A blue banner "TEST CONNECTION" is on all screens. The simulator state becomes "connected". The Dashboard shows about 230 V input. If the simulator is not running, the help is: "Start vlt-simulator.exe, then wait. VoltioVolt connects automatically."
    3. Scenario 1 (key 1, "Power failure now"): the Dashboard shows battery mode (mains off). Open Events: an ON_BATTERY Event with source "test" is there (History and Events show the current source by default). If a Notification Channel is configured, a notification has the prefix "[TEST]". Do not send a Secret. Press 9 or R to return to normal, or go to the next scenario.
    4. Scenario 2 (key 2, "Power failure for 2 minutes, then mains on"): mains goes off, then after 2 minutes mains returns. Watch the Dashboard for both states. An Event for the return (for example MAINS_RESTORED) has source "test".
    5. Scenario 5 (key 5, "Overload: 110 %"): the Dashboard load is 110 %. An overload Event, if any, has source "test". Press 9 or R when you have seen it.
    6. Scenario 6 (key 6, "Battery low: charge 15 % and mains off"): the Dashboard shows a low battery and mains off. If the Shutdown Policy would shut down the computer, a countdown starts and the message at the end is: "Test: the computer would shut down now." The computer does not shut down. Do not turn on "Permit a real computer shutdown in a test".
    7. Scenario 7 (key 7, "Communication loss for 30 seconds"): the simulator sends no response for 30 seconds, then it is normal again. Watch the Dashboard and Events for the loss and the return.
    8. Hidden chart series (RQ-03): open History. Hide one series (click its name in the chart legend). Refresh the data (wait for a new point, or change and restore a filter). Change the time range. Close VoltioVolt and start it again (keep the simulator running, and select "Test connection (simulator)" if needed). The same series stays hidden. The recording of that series did not stop: when you show it again, the points of the hidden period are there.
- Expected result: the blue TEST CONNECTION banner is visible. Scenarios 1, 2, 5, 6 and 7 change the Dashboard as described. Events and Notifications of the test have source "test" and the prefix "[TEST]". The countdown, if it runs, is a dry run: "Test: the computer would shut down now." The computer does not shut down. A hidden History series stays hidden after a refresh, a range change and a restart. No CONTROL bytes go to a real UPS.
- Send: the result of each step (OK or the problem). Do not send a Secret. Screenshots are permitted; check that no Secret, no PIN and no serial number is in them.

### Service mode (later)

These tests need the service mode (vltd, vlt-setup, a local group and a user right). The service mode stays in the source code. It is not in the default package (OD-36). Do not run these tests with the VLT-IS-008 portable package. The text of T-02, T-03, T-04, T-05, T-06 and T-07 is kept here for a later release.

### T-02 vltd development install on Linux

- Purpose: check the systemd unit, the credential, the Vault and the IPC on a real Linux computer.
- Preconditions: Linux with systemd 250 or later. Root access. If possible two runs: one computer with TPM2 and one without TPM2.
- Steps:
    1. Install with docs/user/dev-service.md E.1 and E.2.
    2. `systemctl status vltd`.
    3. `journalctl -u vltd -b --no-pager`.
    4. `cat /etc/vltd/key-mode`.
    5. Restart the service: `systemctl restart vltd`. Do step 3 again.
    6. Add your user to the group vlt. vlt-dev-init.sh does this when you start it with sudo (it adds SUDO_USER). Else: `sudo usermod -a -G vlt <your user>`. Log out and log in again. Then: `id -nG`.
- Expected result: "active (running)". Step 6: the output of `id -nG` contains `vlt`. The log has `vault canary check passed state=available key_mode=host+tpm2` (with TPM2) or `state="reduced protection" key_mode=host` (without TPM2), `audit chain verification ok=true`, `ipc server started`. If no UPS is connected: device state "no device" and no error that stops vltd.
- Send: the output of step 2 and step 4, the log lines of step 3 and step 5 (remove host names if necessary), the systemd version (`systemctl --version`), TPM2 yes or no, and the result of step 6 (yes or no; do not send the user name).

### T-03 vltd development install on Windows with vlt-setup

- Purpose: check vlt-setup (TASK 03-00-00-002) on a real Windows computer.
- Preconditions: Windows 10 or 11, or Windows Server. An Administrator account. The artifact `vltd-vlt-setup-windows-amd64` with the SHA-256 check.
- Steps:
    1. Install with docs/user/dev-service.md E.3. Copy all output of vlt-setup.
    2. `Get-Service vltd` and `sc.exe qprivs vltd` and `sc.exe qfailure vltd`.
    3. `icacls "$env:ProgramData\VoltioVolt"`.
    4. Open %ProgramData%\VoltioVolt\logs\vltd.log.
    5. Uninstall with `.\dev-uninstall.ps1` (keep the data directory). Copy the output.
    6. Install again (step 1). vlt-setup adds the user who started it to the group "VLT Operators" (line "1a."). Sign out and sign in again. Then: `net localgroup "VLT Operators"`.
- Expected result: no vlt-setup step reports an error. Step 6: your user is a member of "VLT Operators". Step 5: dev-uninstall does not delete the group, because it has a member. Status "Running". Required privileges SeShutdownPrivilege and SeChangeNotifyPrivilege. Recovery: restart, restart, none. The ACL has SYSTEM, Administrators and NT SERVICE\vltd only. The log has `vault canary check passed state=available key_mode=dpapi`.
- Send: the output of steps 1, 2, 3 and 5, the log lines of step 4, the Windows version (`winver`), and the result of step 6 (yes or no; do not send the user name).

### T-04 SEC-006 copy test

- Purpose: prove that a copy of the data directory does not open the Vault on a second computer (VLT-RPT-003 7.5).
- Preconditions: T-02 or T-03 is complete on computer A. A second computer B with the same OS.
- Steps: docs/user/dev-service.md E.6.
- Expected result: on B, the Vault state is "locked" (`vault locked reason="host binding failed"`). If the state is "available", stop: this is a security defect.
- Send: the log lines of B with the Vault state, and the OS of A and B.

### T-05 Linux credential with one changed byte

- Purpose: check vltd-fallback.service (TASK 03-00-00-001 step 3).
- Preconditions: T-02 is complete. Make a backup: `cp /etc/credstore.encrypted/vlt-dek /root/vlt-dek.bak`.
- Steps:
    1. `systemctl stop vltd`.
    2. Change one byte in the middle of the file: `printf 'X' | dd of=/etc/credstore.encrypted/vlt-dek bs=1 seek=40 conv=notrunc`.
    3. `systemctl start vltd`. Wait 10 seconds.
    4. `systemctl status vltd vltd-fallback`.
    5. `journalctl -u vltd -u vltd-fallback -b --no-pager | tail -50`.
    6. Restore: `systemctl stop vltd-fallback`, `cp /root/vlt-dek.bak /etc/credstore.encrypted/vlt-dek`, `systemctl reset-failed vltd`, `systemctl start vltd`.
- Expected result: vltd.service is "failed" (243/CREDENTIALS). vltd-fallback.service is "active (running)". The log has `fallback mode: the primary unit failed` and `vault locked reason="primary unit failed"`. After step 6, vltd is "active (running)" with the Vault "available".
- Send: the output of steps 4 and 5, and the state after step 6.

### T-06 Windows DPAPI canary after restarts

- Purpose: check user-scope DPAPI for the virtual account NT SERVICE\vltd (VLT-RPT-003 7.5 item 5; UNVERIFIED in the research).
- Preconditions: T-03 is complete (installed again if you uninstalled it).
- Steps:
    1. `Restart-Service vltd`. Wait 10 seconds.
    2. Find the last `vault canary check` line in vltd.log.
    3. Restart the computer. `Start-Service vltd` (start type manual). Do step 2 again.
- Expected result: both times `vault canary check passed state=available key_mode=dpapi`.
- Send: the two log lines. If the Vault is "locked", also the line with the reason.

### T-07 polkit: CanPowerOff and CanHibernate as the user vltd

- Purpose: find which polkit action variant logind checks for a system service without a session (VLT-RPT-003 7.4 item 7).
- Preconditions: T-02 is complete. The polkit rule /etc/polkit-1/rules.d/50-vltd.rules is installed.
- Steps (as root):
    1. `sudo -u vltd busctl call org.freedesktop.login1 /org/freedesktop/login1 org.freedesktop.login1.Manager CanPowerOff`
    2. `sudo -u vltd busctl call org.freedesktop.login1 /org/freedesktop/login1 org.freedesktop.login1.Manager CanHibernate`
    3. `busctl call org.freedesktop.login1 /org/freedesktop/login1 org.freedesktop.login1.Manager ListInhibitors`
    4. Do not call PowerOff or Hibernate in this test.
- Expected result: step 1 gives `s "yes"`. Step 2 gives `s "yes"` if hibernation is configured, else `"na"` or `"no"` (then vltd uses PowerOff for the hibernate action).
- Send: the output of steps 1 to 3, the distribution and the systemd version.


## T. Türkçe

### Hazırlık

1. Depo Releases sayfasını açın: https://github.com/abuzer1495/voltiovolt/releases
2. "Test build" adlı ön sürümü açın (etiket: test-latest).
3. VoltioVolt-test-windows-amd64.zip veya VoltioVolt-test-linux-amd64.tar.gz dosyasını indirin.
4. SHA-256 değerlerini SHA256SUMS.txt dosyasından kontrol edin (docs/user/probe.md bölüm 1).
5. VoltioVolt'u başlatmadan önce UPSilon 2000'i ve RupsMon hizmetini durdurun. Testlerden sonra gerekiyorsa yeniden başlatın.
6. Windows: zip dosyasını çıkarın ve çıkarılmış klasörden VoltioVolt.exe dosyasını başlatın. Yönetici hakları gerekmez. Microsoft Edge WebView2 Runtime gerekir (Windows 10 ve 11'de vardır). Yöntem 2: install.cmd dosyasını çalıştırın (dosyaları %LOCALAPPDATA%\Programs\VoltioVolt klasörüne kopyalar ve bir Başlat menüsü kısayolu yapmadan önce sorar).
7. Linux: tar.gz dosyasını çıkarın ve ./voltiovolt dosyasını başlatın. Çalışma zamanı paketleri: libgtk-4-1 ve libwebkitgtk-6.0-4 (örneğin Debian ve Ubuntu'da: `sudo apt install libgtk-4-1 libwebkitgtk-6.0-4`). Yöntem 2: install.sh dosyasını çalıştırın (dosyaları ~/.local/opt/voltiovolt klasörüne kopyalar ve bir masaüstü kısayolu yapmadan önce sorar). Kurulum betiği için sudo kullanmayın.
8. VLT-IS-007 testi kurulmuşsa: remove-old-test-service.cmd dosyasını bir kez çalıştırın (bu betik yönetici hakları ister).

Her testte şunlar vardır: amaç, ön koşullar, adımlar, beklenen sonuç ve Instruction Author'a gönderilecekler. Testleri sırayla yapın. Her testin sonuçlarını test biter bitmez gönderin. Secret, PIN, DEK, QID değeri veya seri numarası göndermeyin. Göndermeden önce her dosyayı kontrol edin.

Tüm testler için genel ön koşullar:

- Üretim bilgisayarı değil, bir test bilgisayarı. Her testten önce tüm çalışmalarınızı kaydedin.
- Referans UPS ve USB veya seri kablosu.
- En son "Test build" ön sürümünün (etiket: test-latest) test paketi. Yukarıdaki "Hazırlık" bölümüne bakın.
- Taşınabilir kipte PIN isteğe bağlıdır. PIN belirlenmemişse tüm işlevler kullanılabilir. Secret alanları yalnızca yazılır.
- Testleri şu sırayla yapın: önce T-01, sonra T-08 ile T-17. T-02, T-03, T-05, T-07 ve T-04 ile T-06'nın hizmet kısımları "Hizmet kipi (daha sonra)" bölümündedir.

### T-01 Referans UPS ile vlt-probe Capture

- Amaç: Referans UPS'in Dialect'ini ve gerçek yanıtlarını bulmak (VLT-IS-002 TASK 03-00-00-008).
- Ön koşullar: çıkarılmış test paketindeki vlt-probe (Windows: vlt-probe.exe, Linux: vlt-probe). UPS, USB ile bağlıdır. Mümkünse Linux kullanın, çünkü Windows'ta ham rapor tanımlayıcısı (raw report descriptor) yoktur. UPS bataryası tam doludur.
- Ön koşullar (VLT-IS-006):
    - Testten önce UPSilon 2000'i ve RupsMon hizmetini durdurun. Testten sonra yeniden başlatın.
    - Referans UPS, sanal bir COM portunda seri Megatec (Q1, F, I) kullanır (OD-26). Windows'ta "--transport serial --port COM3" kullanın. Linux'ta portu "vlt-probe scan" ile bulun (/dev/ttyUSB* veya /dev/ttyACM*).
    - Portun donanım kimliğini yazın (Windows Aygıt Yöneticisi: Bağlantı Noktaları, COM3, Özellikler, Ayrıntılar, Donanım Kimlikleri).
- Adımlar:
    1. docs/user/probe.md belgesini izleyin: `vlt-probe scan`, sonra `vlt-probe probe --transport hid` (Linux: sudo ile).
    2. docs/user/probe.md bölüm 4'teki şebeke testini yapın (watch modu, 3 dakika, giriş gücü 60 saniye veya daha kısa süre kapalı). İki zamanı not edin.
    3. UPS'in seri portu varsa: 1. adımı `--transport serial --port <port>` ile tekrarlayın.
- Beklenen sonuç: her çalıştırma için bir Capture dosyası. probe Capture dosyasında Q1 veya QS için bir yanıt vardır. watch Capture dosyası, giriş gücü kapalıyken durum biti 7 = 1 gösterir.
- Gönderin: Capture dosyaları, iki zaman, işletim sistemi adı ve sürümü, `scan` çıktısındaki USB vendor ve product ID değerleri.

### T-08 Politika Simülasyonu

- Amaç: varsayılan politikanın ve değiştirilmiş bir politikanın Simülasyonunu (VLT-IS-004 TASK 03-00-00-006) VoltioVolt ile kontrol etmek. Ayrıca Simülasyon adımı "İşletim sistemi kapatması"nı kontrol etmek (VLT-IS-008 TASK 03-00-00-004).
- Ön koşullar: Test paketi "Hazırlık" bölümündeki gibi hazırlanmıştır. UPS gerekmez. Bu test için UPS bağlamayın: Simülasyon bilgisayarı kapatmaz, ancak etkin bir politika, UPS bir elektrik kesintisi bildirdiğinde bilgisayarı kapatabilir. Grup üyeliği ve yönetici hakları gerekmez.
- Adımlar:
    1. VoltioVolt'u çıkarılmış klasörden başlatın (Windows: VoltioVolt.exe, Linux: ./voltiovolt). İlk çalıştırma sihirbazı başlarsa, PIN adımı isteğe bağlıdır: bir "Atla" düğmesi vardır. Metin şudur: "Bir PIN, bu bilgisayardaki diğer kişilerin kapatma ayarlarını değiştirmesini önler." "Atla" düğmesine tıklayabilir veya bir PIN belirleyebilirsiniz. PIN belirlenmişse PIN ile kilidi açın. PIN belirlenmemişse tüm işlevler kullanılabilir.
    2. "Kapatma Politikası" ekranını açın. Sürümü ve durumu (etkin veya etkin değil) not edin. Ekran "Bu kullanıcı bilgisayarı kapatamaz." hatasını gösteriyorsa bunu not edin. Bu durumda politikayı etkinleştiremezsiniz.
    3. "Simülasyonu çalıştır" düğmesine tıklayın. Her senaryonun sonucunu not edin (başarılı veya başarısız ve kapatma isteğinin veya iptalin zamanı). "İşletim sistemi kapatması" adımının sonucunu not edin (başarılı, veya 2. adımdaki aynı hata).
    4. Simülasyon başarılıysa ("İşletim sistemi kapatması" dahil), "Etkinleştir" düğmesine tıklayın. Onay metnini okuyun ve onaylayın. Durum "etkin" olur.
    5. "Düzenle" düğmesine tıklayın. "countdown_seconds" değerini 90 yapın. "Kaydet" düğmesine tıklayın.
    6. "Politika artık etkin değil. Etkinleştirmek için Simülasyonu çalıştırın." metnini ve "etkin değil" durumunu kontrol edin. "Etkinleştir" kapalıdır.
    7. "Simülasyonu çalıştır" düğmesine yeniden tıklayın. Sonra "Etkinleştir" açık olur. Etkinleştirmeyin. "Düzenle" düğmesine tıklayın, "countdown_seconds" değerini yeniden 60 yapın ve "Kaydet" düğmesine tıklayın.
- Beklenen sonuç: 3. adım: tüm senaryolar "başarılı" ve "İşletim sistemi kapatması" adımı başarılı. Varsayılan politika: power_fail_long 370 s'de bir kapatma isteği verir, power_fail_short bir iptal verir, low_battery reddedilen bir iptal ve 90 s'de bir istek verir, comm_lost_on_battery sınırından önce bir istek verir. 6. adım: değişiklikten sonra politika etkin değildir. 7. adım: "Etkinleştir" yalnızca yeni Simülasyon başarılı olduktan sonra açıktır. "İşletim sistemi kapatması" başarısızsa hata tam olarak "Bu kullanıcı bilgisayarı kapatamaz." olur ve politikayı etkinleştiremezsiniz.
- Gönderin: 2, 3, 6 ve 7. adımların sonuçları (ekran görüntülerine izin vardır; içlerinde Secret olmadığını kontrol edin), VoltioVolt sürümü (Genel, Hakkında) ve işletim sistemi.

### T-09 Referans UPS ile gerçek kapatma

UYARI: Bu test bilgisayarı kapatır. Tüm çalışmalarınızı kaydedin. Tüm uygulamaları kapatın. UPS bataryasının tam dolu olduğundan emin olun. trigger.on_battery_seconds değerini 30 yapın. UPS giriş gücünü kesin. Bilgisayar kapandıktan sonra giriş gücünü yeniden bağlayın.

- Amaç: gerçek donanımda tam zinciri kontrol etmek: UPS, Aygıt Yöneticisi, Olay Motoru, Politika Motoru, işletim sistemi kapatma yürütücüsü (TASK 03-00-00-006 ve 007).
- Ön koşullar: T-01 ve T-08 tamamlanmıştır. Bilgisayar, Referans UPS tarafından beslenir ve UPS bilgisayara USB veya seri ile bağlıdır. VoltioVolt çalışır (T-08'deki gibi çıkarılmış klasörden başlatılmıştır). Bilgisayarı başka bir kişi kullanmaz.
- Adımlar (tümü VoltioVolt'ta):
    1. PIN belirlenmişse PIN ile kilidi açın. "Kapatma Politikası" ekranını açın, "Düzenle" düğmesine tıklayın, "trigger.on_battery_seconds" değerini 30 yapın (diğer varsayılanlar kalsın: geri sayım 60 s, eylem kapatma) ve "Kaydet" düğmesine tıklayın.
    2. "Simülasyonu çalıştır" düğmesine tıklayın. Sonuç, "İşletim sistemi kapatması" adımı dahil, "başarılı" olmalıdır. Sonra "Etkinleştir" düğmesine tıklayın ve onaylayın.
    3. "Aygıt" ekranını açın: durum "bağlı" olmalıdır. Linux "port izni yok" veya "HID izni yok" gösteriyorsa yardım metnini izleyin (komutu bir kez çalıştırın, sonra oturumu kapatıp yeniden açın). VoltioVolt bu komutu çalıştırmaz ve bir udev kuralı kurmaz.
    4. "Gösterge paneli" ekranını açın. Kip "Şebeke elektriği"dir. Zamanı not edin.
    5. UPS giriş gücünü kesin. Zamanı not edin.
    6. Gösterge panelini izleyin: kip "Aküde" olur, ON_BATTERY Olayı Olayların en üstündedir, 30 saniye sonra 60 saniyelik geri sayım bandı başlar.
    7. Bilgisayar kapanır. Zamanı not edin.
    8. UPS giriş gücünü yeniden bağlayın. Bilgisayarı başlatın.
    9. VoltioVolt'u yeniden başlatın. Windows: veri klasöründeki (Genel, "Veri klasörü") günlük dosyasını açın: logs\vltd.log, ve Olay Görüntüleyicisi, Sistem günlüğü, kaynak User32, olay kimliği 1074. Linux: "Güvenlik" ekranını açın ve denetim kaydını okuyun; VoltioVolt'u bir uçbirimden başlattıysanız standart hata çıktısını da saklayın. `journalctl -u vltd` kullanmayın.
    10. PIN belirlenmişse kilidi açın. "Güvenlik" ekranını açın ve denetim kaydını okuyun (policy_countdown_start, policy_shutdown_request, os_shutdown_request). Sonra "Kapatma Politikası"nda "trigger.on_battery_seconds" değerini yeniden 300 yapın.
- Beklenen sonuç: bilgisayar, 5. adımdan yaklaşık 90 saniye sonra, bir iletişim kutusu olmadan kapanır. Denetim kaydında üç girdi vardır. Windows: olay 1074, 0x6000c neden kodunu gösterir ("Power Failure: Environment"; tam olay metni UNVERIFIED'dir).
- İptal: geri sayımı durdurmak için, bitmeden önce giriş gücünü yeniden bağlayın (cancel_when_mains_returns açıktır), veya geri sayım bandındaki "İptal" düğmesine tıklayın (LOW_BATTERY ile mümkün değildir).
- Gönderin: 4, 5 ve 7. adımların zamanları, günlük satırları, denetim girdileri (içlerinde Secret yoktur) ve Windows olay 1074 metni.

### T-10 VLT GUI testi

- Amaç: Owner bilgisayarında VoltioVolt içindeki VLT GUI'yi kontrol etmek (VLT-IS-005) ve tüm Türkçe metni gözden geçirmek (VLT-IS-005 TASK 03-00-00-008 adım 1.4). Ayrıca VLT-IS-008'in taşınabilir işlevlerini kontrol etmek: veri klasörü, oturum açılışında başlatma, tepsi ve isteğe bağlı PIN.
- Ön koşullar: T-01 ve T-08 tamamlanmıştır (OD-23, taşınabilir sıra). VoltioVolt, SHA-256 kontrolü yapılmış test paketindendir. Yeni bir ilk başlatma için: yeni bir veri klasörü, PIN ile veya PIN olmadan (isteğe bağlı).
- Adımlar:
    1. VoltioVolt'u çıkarılmış klasörden başlatın. Motor ve VLT GUI tek süreçte çalışır. Pencereyi kapatın: pencere gizlenir ve izleme devam eder. İlk kez şu metin görünür: "VoltioVolt, UPS izlemeye tepside devam eder." Pencereyi tepside açın. Sonra tepsi menüsündeki "Çık"ı kullanın: onay metni "Çıkarsanız VoltioVolt izlemeyi ve otomatik kapatmayı durdurur." Onaylayın. Uygulama durur. Yeniden başlatın.
    2. PIN belirleme (isteğe bağlı): yeni bir ilk başlatmada sihirbazın PIN adımında "Atla" vardır ve metin "Bir PIN, bu bilgisayardaki diğer kişilerin kapatma ayarlarını değiştirmesini önler." 5 karakterli bir PIN deneyin (reddedilir), sonra doğru bir PIN belirleyin veya "Atla" düğmesine tıklayın. Genel ekranında, PIN yoksa "PIN belirle", PIN varsa "PIN kaldır" vardır (Kaldır için geçerli PIN gerekir).
    3. Giriş (yalnızca PIN belirlenmişse): "Kilidi aç" düğmesine tıklayın ve GUI blok süresini gösterene kadar yanlış PIN girin. Bekleyin, sonra doğru PIN ile kilidi açın.
    4. Kilitleme (yalnızca PIN belirlenmişse): "Kilitle" düğmesine tıklayın. GUI "Kilitli" gösterir ve Gösterge panelini göstermeye devam eder. Kilidi yeniden açın. Sonra tıklamadan boşta kalma süresini (auth.session_idle_minutes) bekleyin: GUI kendisini kilitler.
    5. Gösterge paneli değerleri: değerleri (giriş ve çıkış gerilimi, frekans, yük, batarya şarjı ve gerilimi, çalışma süresi, sıcaklık) UPS göstergesi veya UPS belgelendirmesi ile karşılaştırın.
    6. Aygıt ekranı: durum "bağlı", Transport, Dialect, model verileri. Seri numarası bir raporda gösterilmez.
    7. Geçmiş grafikleri: her aralığı (1 saat ile 1 yıl) ve her grafiği açın. Bir grafiğin tablo görünümünü açın.
    8. Olaylar: daha eski Olayları yükleyin, şiddet ve türe göre süzün.
    9. Politika düzenleme: aralık dışında bir değer değiştirin (örneğin countdown_seconds 5): GUI, göndermeden önce bunu reddeder. Doğru bir değer yapın ve kaydedin.
    10. Simülasyon: çalıştırın ve her senaryonun adımlarını ("İşletim sistemi kapatması" dahil), sanal zamanlarını ve "başarılı" veya "başarısız" sonucunu kontrol edin.
    11. Etkinleştirme: "Etkinleştir" yalnızca başarılı bir Simülasyondan sonra açıktır ("İşletim sistemi kapatması" dahil). Etkinleştirin, sonra onay ile devre dışı bırakın.
    12. Dil ve Genel: "Genel" ekranını açın. Ekran veri klasörü yolunu gösterir. "Oturum açılışında başlat" anahtarı varsayılan olarak kapalıdır. Açın, sonra kapatın: VoltioVolt yalnızca kullanıcının başlangıç klasörünü kullanır. "Türkçe", sonra "English" seçin. VoltioVolt'u kapatın ve yeniden başlatın: dil kalır.
    13. Tüm Türkçe metnin gözden geçirilmesi: "Türkçe" seçin ve görebildiğiniz her ekranı ve her iletiyi açın. Türkçe karakterleri (ç, ğ, ı, İ, ö, ş, ü), sözcükleri ve dilbilgisini kontrol edin.
    14. Klavye: bir ekranda yalnızca Tab, Shift+Tab, Enter ve Space tuşlarını kullanın. Odak her zaman görünürdür.
    15. Tema: işletim sistemi temasını açık ve koyu arasında değiştirin. GUI onu izler.
- Beklenen sonuç: tüm adımlar yazıldığı gibi çalışır. GUI'deki hiçbir metin yanlış dilde değildir ve yanlış Türkçe karakter içermez.
- Gönderin: her adımın sonucu (tamam veya sorun), değiştirilmesi gereken Türkçe metinlerin listesi (ekran, mevcut metin ve doğru metin), işletim sistemi, veri klasörü yolu ve VoltioVolt sürümü (Genel, Hakkında). Ekran görüntülerine izin vardır; içlerinde Secret, PIN ve seri numarası olmadığını kontrol edin.

### T-11 UPS Kontrolü (VLT-IS-006)

- Amaç: UPS Kontrolü ekranını ve Referans UPS'te CONTROL komutları test ve sesli uyarıyı kontrol etmek (VLT-IS-006 TASK 03-00-00-005, 007 ve 008).
- Ön koşullar: T-01, T-08 ve T-10 tamamlanmıştır. VoltioVolt, `device.transport` seri ve T-01'deki port ile çalışır ("Aygıt" ekranı). Aygıt durumu "bağlı"dır. Şebeke elektriği vardır. UPS bataryası tam doludur.
- Adımlar (tümü VoltioVolt'ta):
    1. PIN belirlenmişse PIN ile kilidi açın. "UPS Kontrolü" ekranını açın. Düğmeler açıktır.
    2. "Sesli uyarı kapalı" düğmesine tıklayın. Zamanı not edin. UPS giriş gücünü 10 saniye kesin: UPS ses çıkarmaz. Giriş gücünü yeniden bağlayın.
    3. "Sesli uyarı açık" düğmesine tıklayın. UPS giriş gücünü 10 saniye kesin: UPS ses çıkarır. Giriş gücünü yeniden bağlayın.
    4. "10 saniye test" düğmesine tıklayın, sonra "Testi başlat" ile onaylayın. Zamanı not edin. TEST_STARTED Olayı görünür. Testten sonra ekran, son test sonucunu (TEST_PASSED veya TEST_FAILED) zamanı ile gösterir.
    5. "Olaylar" ekranını açın: TEST_STARTED ve onun temizlenmesi.
    6. "Güvenlik" ekranını açın ve denetim kaydını okuyun: ups_beeper_set, ups_control, ups_control_verify, ups_test_start ve ups_test_end.
- Beklenen sonuç: her komut "doğrulandı"dır. Test sonucu TEST_PASSED'dir. Denetim kaydında 6. adımın girdileri, baytlarla (Q, T) birlikte vardır.
- Gönderin: zamanlar, test sonucu, denetim girdileri (içlerinde Secret yoktur) ve Olaylar.

### T-12 UPS çıkışını kapatma ve yeniden açma (VLT-IS-006)

UYARI: Bu test sırasında UPS çıkışını kapatır. UPS'e bağlı tüm cihazların gücü kesilir. UPS'e yalnızca test bilgisayarını bağlayın. Tüm çalışmalarınızı kaydedin. Bataryanın tam dolu olduğundan emin olun.

- Amaç: Kapatma Politikasının UPS çıkışını kapatma adımını kontrol etmek (VLT-IS-006 TASK 03-00-00-006, OD-28).
- Ön koşullar: T-09 ve T-11 tamamlanmıştır. UPS çıkışına yalnızca test bilgisayarı bağlıdır. VoltioVolt, T-01'deki port ile çalışır.
- Adımlar (tümü VoltioVolt'ta):
    1. PIN belirlenmişse PIN ile kilidi açın. "Kapatma Politikası" ekranını açın, "Düzenle" düğmesine tıklayın, ups_off.enabled açık, ups_off.delay_minutes 2, ups_off.restore_minutes 1 yapın (T-09'daki trigger.on_battery_seconds 30 kalsın) ve "Kaydet" düğmesine tıklayın.
    2. Simülasyonu çalıştırın. Sonuç "başarılı" olmalıdır ("İşletim sistemi kapatması" dahil) ve her senaryo "UPS çıkışını kapatma komutu (deneme, gönderilmedi)" adımını gösterir. Politikayı etkinleştirin.
    3. UPS giriş gücünü kesin. Zamanı not edin.
    4. İşletim sisteminin kapanma, UPS çıkışının kapanma ve UPS çıkışının açılma zamanlarını kaydedin.
    5. 1. çalıştırma: giriş gücünü yeniden açma süresinden önce (UPS çıkışı yeniden açılmadan önce) bağlayın.
    6. 2. çalıştırma: 3. ve 4. adımları tekrarlayın ve giriş gücünü bağlamayın. UPS'in ne yaptığını not edin. Sonra giriş gücünü bağlayın.
    7. Bilgisayarı başlatın. VoltioVolt'u başlatın. "Güvenlik" ekranını açın ve denetim kaydını okuyun: policy_countdown_start, policy_ups_off, policy_shutdown_request, os_shutdown_request.
    8. ups_off.enabled ayarını yeniden kapatın (varsayılan, OD-28), Simülasyonu çalıştırın ve politikayı etkinleştirin.
- Beklenen sonuç: işletim sistemi, UPS çıkışı kapanmadan önce kapanır (kapatma başladıktan 2 dakika sonra). 1. çalıştırma: UPS çıkışı kapandıktan 1 dakika sonra açılır. 2. çalıştırma: giriş gücü olmadan davranış UNVERIFIED'dir (VLT-REF-001 5.1 madde 4); bunu not edin.
- Gönderin: her iki çalıştırma için 3. ve 4. adımların zamanları, denetim girdileri ve günlük satırları (Windows: veri klasöründeki günlük dosyası, logs\vltd.log).

### T-13 Bildirimler (VLT-IS-007)

- Amaç: masaüstü, Telegram ve e-posta Kanallarını kontrol etmek (VLT-IS-007 TASK 03-00-00-004, 005 ve 006).
- Ön koşullar: T-09 tamamlanmıştır. Test paketindeki VoltioVolt çalışır. Bir Telegram bot belirteci ve bir Gmail uygulama parolası (Owner'ın). Belirteci veya parolayı göndermeyin.
- Adımlar (tümü VoltioVolt'ta, "Bildirimler" ekranı, aksi belirtilmedikçe):
    1. Masaüstü: VoltioVolt'u başlatın. UPS giriş gücünü kesin. Bir masaüstü bildirimi görünür. Giriş gücünü bağlayın. Zamanı not edin.
    2. Telegram: bot belirtecini girin (yalnızca yazma). "Sohbet eşleştir" düğmesine tıklayın. Telegram'dan bota `/start <kod>` gönderin. `/durum` gönderin. UPS giriş gücünü kesin. Bir Telegram Bildirimi gelir. Giriş gücünü bağlayın. Zamanı not edin.
    3. E-posta: "Gmail" hazır ayarını seçin. Gmail adresini ve uygulama parolasını girin (yardım metni: "Normal parolanızı değil, bir Google uygulama parolası kullanın. Google hesabı, Güvenlik, 2 Adımlı Doğrulama, Uygulama parolaları."). "Test mesajı gönder" düğmesine tıklayın. UPS giriş gücünü kesin. Bir e-posta Bildirimi gelir. Giriş gücünü bağlayın. Zamanı not edin.
- Beklenen sonuç: bir masaüstü bildirimi, bir Telegram iletisi (`/durum` yanıtı UPS durumunu içerir) ve bir e-posta, her biri güç kesintisi sırasında. Test e-postası gelir.
- Gönderin: zamanlar ve her Kanalın çalıştığına dair onay. Belirteci, parolayı veya ileti metnini göndermeyin (durum verisi içerebilir).


### T-14 PIN belirleme, kilitleme, kilit açma, yeniden başlatma ve PIN'i unuttum (VLT-IS-009)

- Amaç: doğru PIN ile kilidin açıldığını (yeniden başlatmadan sonra da) ve "PIN'i unuttum" işlevini kontrol etmek (VLT-IS-009 TASK 03-00-00-001 ve 002, OD-40, R-02).
- Ön koşullar: Test paketindeki VoltioVolt taşınabilir kipte çalışır. PIN belirleyebileceğiniz bir veri klasörü kullanın. PIN'i göndermeyin.
- Adımlar (tümü VoltioVolt'ta):
    1. PIN belirleme: PIN yoksa "Genel" ekranında "PIN belirle" düğmesine tıklayın veya kurulum sihirbazının PIN adımını kullanın. En az 6 karakterlik bir PIN belirleyin. Daha önceki bir testten PIN varsa onu kullanabilirsiniz.
    2. Kilitleme: "Kilitle" düğmesine tıklayın. Arayüz "Kilitli" gösterir ve Gösterge paneli görünür kalır.
    3. Kilit açma: "Kilidi aç" düğmesine tıklayın, doğru PIN'i girin ve giriş ekranındaki "Kilidi aç" düğmesine tıklayın. Arayüzün kilidi açılır. Bu, R-02 işlevidir.
    4. Yeniden başlatma: tepsi menüsünden "Çık"ı kullanın, onaylayın ve VoltioVolt'u aynı klasörden ve aynı veri klasörüyle yeniden başlatın.
    5. Yeniden başlatmadan sonra kilit açma: aynı doğru PIN'i girin ve "Kilidi aç" düğmesine tıklayın. Arayüzün kilidi açılır.
    6. PIN'i unuttum: "Kilitle" düğmesine tıklayın, sonra "PIN'i unuttum" bağlantısına tıklayın. Metin şudur: "Bu işlem PIN'i kaldırır ve kayıtlı tüm parolaları ve belirteçleri (Telegram, e-posta) siler. Otomatik kapatma devre dışı kalır. Ayarlarınız ve geçmişiniz kalır." Onay metni şudur: "Onaylamak için RESET yazın." RESET yazın (bu sözcük İngilizce ve Türkçede aynıdır). "PIN'i sıfırla" düğmesine tıklayın.
    7. Sıfırlamadan sonra: kurulum sihirbazı PIN adımında başlar. Kapatma Politikası devre dışıdır. Ayarlar, Olaylar ve Geçmiş kalır. Kayıtlı parolalar ve belirteçler silinir. "Güvenlik" ekranını açın ve sıfırlamanın denetim kaydını bulun.
- Beklenen sonuç: 3. ve 5. adımlarda doğru PIN ile kilit açılır. 6. adım PIN'i kaldırır, Gizli bilgileri siler ve politikayı devre dışı bırakır. Ayarlar ve geçmiş kalır. Onay sözcüğü RESET'tir.
- R-08 düzeltmesinden sonraki Owner sırası (VLT-IS-011 TASK 03-00-00-001, OD-50, OD-51, OD-52). Bu adımları önce, yeni bir veri klasörüyle yapın:
    1.1. VoltioVolt'u yeni bir veri klasörüyle başlatın. Kurulum sihirbazı PIN adımında başlar.
    1.2. En az 6 karakterlik bir PIN girin ve "PIN belirle" düğmesine tıklayın.
    1.3. Sihirbaz "UPS portu" adımını açar. VoltioVolt "Kilitli" göstermez ve PIN istemez.
    1.4. "Test bağlantısı (simülatör)" düğmesine tıklayın. Ayar kaydedilir ve adım seçilen portu gösterir. VoltioVolt PIN istemez.
    1.5. "İleri" ile Kapatma Politikası adımına gidin. "Simülasyonu çalıştır", sonra "Etkinleştir" düğmesine tıklayın ve onaylayın. VoltioVolt PIN istemez.
    1.6. Sihirbazın sonuna gidin ve "Bitir" düğmesine tıklayın. Gösterge paneli açılır ve VoltioVolt'un kilidi açıktır.
    1.7. "Kilitle" düğmesine tıklayın: kilit ekranı açılır. PIN'i girin ve "Kilidi aç" düğmesine tıklayın: VoltioVolt'un kilidi açılır.
    1.3–1.7 adımlarını, PIN'i sihirbaz yerine "Genel" ekranında ("PIN belirle") belirleyerek yineleyin: "PIN belirle"den sonra VoltioVolt'un kilidi açık kalır. Kilit ekranı sihirbazın üstünü hiçbir zaman örtmez: bir sihirbaz adımı PIN gerektirirse PIN alanı sihirbaz penceresinin içindedir. VoltioVolt, 10 dakika klavye veya fare kullanılmazsa kilitlenir; canlı değerler kilidi açık tutmaz.
- Gönderin: her adımın sonucu (tamam veya sorun) ve denetim kaydı. PIN'i göndermeyin. Ekran görüntülerine izin vardır; PIN ve Gizli bilgi olmadığından emin olun.

### T-15 1280 × 720 boyutunda sihirbaz ve ekranlar (VLT-IS-009)

- Amaç: ilk açılış penceresini, kompakt sihirbazı ve tüm ekranları 1280 × 720 boyutunda ve Owner'ın ekran ölçeğinde kontrol etmek (VLT-IS-009 TASK 03-00-00-003, R-04, OD-41).
- Ön koşullar: Test paketindeki VoltioVolt. İlk açılış penceresi için yeni bir veri klasörü kullanın (örneğin yeni boş bir klasör, veri klasörü seçeneğiyle veya taze bir çıkarımla). Owner bilgisayarının ekran ölçeğini not edin (Windows: Ayarlar, Sistem, Ekran, Ölçek; örneğin %100, %125 veya %150).
- Adımlar:
    1. İlk açılış: pencere "Kurulum sihirbazı başlatılsın mı?" metni ile "Başlat" ve "Daha sonra" düğmelerini gösterir. "Daha sonra"ya tıklayın: Gösterge paneli açılır. VoltioVolt'tan çıkın ve yeniden başlatın: pencere yine görünür. "Daha sonra"ya ikinci kez tıklayın: pencere bir daha görünmez. "Genel" ekranında "Kurulum sihirbazını başlat" düğmesi durur.
    2. Sihirbaz boyutu: yeni bir veri klasörüyle veya Genel ekranından "Başlat" veya "Kurulum sihirbazını başlat" düğmesine tıklayın. Sihirbaz 760 × 560 piksellik bir penceredir. Geri, Atla ve İleri (veya Bitir) düğmeleri sağ alttadır. Her adımdan geçin (PIN, UPS portu, canlı değerler, Kapatma Politikası, Telegram, e-posta, modem ve yönlendirici, Özet). 1280 × 720 boyutunda her adım kaydırma çubuğu olmadan sığar. Alanların yardım satırları ve örnek değerleri görünür.
    3. 1280 × 720 ekranlar: sihirbazdan sonra (Bitir veya Daha sonra) her ekranı 1280 × 720 boyutunda açın: Gösterge paneli, Aygıt, Geçmiş, Olaylar, Kapatma Politikası, UPS Kontrolü, Bildirimler, Güvenlik, Genel. Her ekran kullanılabilir: önemli bir denetim kesilmez.
    4. Owner'ın ekran ölçeği: Owner bilgisayarının olağan ölçeğini koruyun. 3. adımı (ve mümkünse 2. adımı) yineleyin. Ölçeği not edin.
- Beklenen sonuç: ilk açılış penceresi 1. adımdaki gibi çalışır. Her sihirbaz adımı kaydırma çubuğu olmadan 760 × 560 piksele sığar. Ekranlar 1280 × 720 boyutunda ve Owner'ın ekran ölçeğinde çalışır.
- Gönderin: her adımın sonucu, ekran ölçeği ve işletim sistemi. Ekran görüntülerine izin vardır; Gizli bilgi, PIN ve seri numarası olmadığından emin olun. Bir adımda veya ekranda kaydırma çubuğu veya kesilmiş bir denetim varsa bunu yazın ve o adımın veya ekranın görüntüsünü gönderin.

### T-16 Demo kipi (VLT-IS-009)

- Amaç: Demo (simülatör) taşımasını kontrol etmek (VLT-IS-009 TASK 03-00-00-005, OD-43). Bu test gerçek bir UPS kullanmaz. Bilgisayarı kapatmamalı ve gerçek bir UPS'e CONTROL baytı göndermemelidir.
- Ön koşullar: Test paketindeki VoltioVolt çalışır. PIN varsa PIN ile kilidi açın. Gerçek bir UPS gerekli değildir.
- Adımlar (tümü VoltioVolt'ta):
    1. "Aygıt" ekranını (veya sihirbazın UPS portu adımını) açın. Taşıma olarak "Demo (simülatör)" seçin. Tüm ekranlarda sarı bir "DEMO" bandı görünür.
    2. Gösterge panelini açın: değerler yavaş değişir. "Olaylar" ve "Geçmiş" ekranlarını açın: demo Olayları ve ölçümleri varsayılan olarak gizlidir. "Diğer kaynakları göster" kutuları kapalıdır. Onları açın: demo verisi görünür ve "demo" olarak işaretlidir.
    3. Aygıt ekranında "Güç kesintisini simüle et" düğmesine tıklayın. Simülatör 60 saniye bataryaya geçer, sonra şebeke elektriği döner.
    4. Kapatma Politikası bilgisayarı kapatacak olsa, ileti şudur: "Demo: bilgisayar şimdi kapanırdı." Bilgisayar kapanmaz.
    5. "UPS Kontrolü" ekranını açın ve kısa bir test başlatın veya hoparlörü değiştirin. Komut bir kuru çalıştırmadır: gerçek bir UPS'e CONTROL baytı gitmez ve bilgisayar değişmez.
    6. Bir Bildirim Kanalı yapılandırılmışsa, bildirimin ön eki "[DEMO] " olur. Gizli bilgi göndermeyin.
    7. Demo kipinden çıkış: farklı bir taşıma veya gerçek bir port seçin. Onay metni şudur: "Demo modundan çıkılsın ve gerçek bir UPS bağlantısı kullanılsın mı? Demo olayları ve ölçümleri geçmişte kalır ve demo olarak işaretlenir." İsterseniz onaylayın veya iptal edin. Demo geçmişi kalır ve demo olarak işaretlidir.
- Beklenen sonuç: sarı DEMO bandı görünür. Değerler yavaş değişir. "Güç kesintisini simüle et" 60 saniye sürer. Bilgisayar kapanmaz. Gerçek bir UPS'e CONTROL baytı gitmez. Demo Olayları ve ölçümleri varsayılan olarak gizlidir ve "demo" olarak işaretlidir. Çıkış onayı 7. adımdaki tam metindir.
- Gönderin: her adımın sonucu (tamam veya sorun). Gizli bilgi göndermeyin. Ekran görüntülerine izin vardır; Gizli bilgi, PIN ve seri numarası olmadığından emin olun.

### T-17 Simülatör testi (VLT-IS-010)

- Amaç: UPS simülatörünü ve test bağlantısını kontrol etmek (VLT-IS-010, RQ-01, RQ-02, RQ-03). Bu test gerçek bir UPS kullanmaz. Bilgisayarı kapatmamalıdır (`test.real_shutdown` kapalı kalsın) ve gerçek bir UPS'e CONTROL baytı göndermemelidir.
- Ön koşullar: Test paketi çıkarılmıştır. vlt-simulator, VoltioVolt ile aynı klasördedir (Windows'ta `vlt-simulator.exe`, Linux'ta `./vlt-simulator`). Gerçek bir UPS gerekli değildir. PIN varsa PIN ile kilidi açın. "Bir testte gerçek bilgisayar kapanmasına izin ver" ayarını kapalı bırakın. Ekran, tuşlar, menü ve senaryolar için docs/user/simulator.md dosyasına bakın.
- Adımlar:
    1. Simülatörü başlatın (Windows: bir cmd penceresinde `vlt-simulator.exe`; Linux: bir uçbirimde `./vlt-simulator`). "VoltioVolt UPS Simülatörü" ekranı açılır. Durum "bekliyor"dur.
    2. VoltioVolt'u başlatın. Aygıt ekranını açın ve "Test bağlantısı (simülatör)" seçin. Tüm ekranlarda mavi bir "TEST BAĞLANTISI" bandı görünür. Simülatör durumu "bağlı" olur. Gösterge paneli yaklaşık 230 V giriş gösterir. Simülatör çalışmıyorsa yardım metni şudur: "vlt-simulator.exe uygulamasını başlatın, sonra bekleyin. VoltioVolt otomatik bağlanır."
    3. Senaryo 1 (1 tuşu, "Şimdi elektrik kesintisi"): Gösterge paneli batarya kipini gösterir (şebeke kapalı). Olaylar ekranını açın: kaynağı "test" olan bir ON_BATTERY Olayı vardır (Geçmiş ve Olaylar varsayılan olarak güncel kaynağı gösterir). Bir Bildirim Kanalı yapılandırılmışsa, bildirimin ön eki "[TEST]" olur. Gizli bilgi göndermeyin. Normal değerlere dönmek için 9 veya R tuşuna basın, veya sonraki senaryoya geçin.
    4. Senaryo 2 (2 tuşu, "2 dakikalık kesinti, sonra şebeke açık"): şebeke kapanır, 2 dakika sonra şebeke döner. Her iki durumu da Gösterge panelinde izleyin. Dönüşe ait bir Olay (örneğin MAINS_RESTORED) "test" kaynağına sahiptir.
    5. Senaryo 5 (5 tuşu, "Aşırı yük: % 110"): Gösterge panelindeki yük % 110 olur. Varsa bir aşırı yük Olayı "test" kaynağına sahiptir. Gördükten sonra 9 veya R tuşuna basın.
    6. Senaryo 6 (6 tuşu, "Akü düşük: şarj % 15 ve şebeke kapalı"): Gösterge paneli düşük akü ve kapalı şebeke gösterir. Kapatma Politikası bilgisayarı kapatacak olsa, bir geri sayım başlar ve sonundaki ileti şudur: "Test: bilgisayar şimdi kapanırdı." Bilgisayar kapanmaz. "Bir testte gerçek bilgisayar kapanmasına izin ver" ayarını açmayın.
    7. Senaryo 7 (7 tuşu, "30 saniyelik haberleşme kaybı"): simülatör 30 saniye yanıt göndermez, sonra yeniden normaldir. Kaybı ve dönüşü Gösterge panelinde ve Olaylar ekranında izleyin.
    8. Gizli grafik serisi (RQ-03): Geçmiş ekranını açın. Bir seriyi gizleyin (grafik gösterimindeki adına tıklayın). Veriyi yenileyin (yeni bir nokta bekleyin, veya bir süzgeci değiştirip geri alın). Zaman aralığını değiştirin. VoltioVolt'u kapatın ve yeniden başlatın (simülatörü çalışır durumda bırakın ve gerekirse "Test bağlantısı (simülatör)" seçin). Aynı seri gizli kalır. O serinin kaydı durmamıştır: yeniden gösterdiğinizde, gizli olduğu sürenin noktaları vardır.
- Beklenen sonuç: mavi TEST BAĞLANTISI bandı görünür. 1, 2, 5, 6 ve 7. senaryolar Gösterge panelini anlatıldığı gibi değiştirir. Test Olayları ve Bildirimleri "test" kaynağına ve "[TEST]" ön ekine sahiptir. Geri sayım çalışırsa kuru çalıştırmadır: "Test: bilgisayar şimdi kapanırdı." Bilgisayar kapanmaz. Gizli bir Geçmiş serisi, yenilemeden, aralık değişiminden ve yeniden başlatmadan sonra gizli kalır. Gerçek bir UPS'e CONTROL baytı gitmez.
- Gönderin: her adımın sonucu (tamam veya sorun). Gizli bilgi göndermeyin. Ekran görüntülerine izin vardır; Gizli bilgi, PIN ve seri numarası olmadığından emin olun.

### Hizmet kipi (daha sonra)

Bu testler hizmet kipini (vltd, vlt-setup, yerel bir grup ve bir kullanıcı hakkı) gerektirir. Hizmet kipi kaynak kodunda kalır. Varsayılan pakette yoktur (OD-36). Bu testleri VLT-IS-008 taşınabilir paketi ile çalıştırmayın. T-02, T-03, T-04, T-05, T-06 ve T-07 metinleri daha sonraki bir sürüm için burada tutulur.

### T-02 Linux'ta vltd geliştirme kurulumu

- Amaç: systemd birimini, kimlik bilgisini (credential), Vault'u ve IPC'yi gerçek bir Linux bilgisayarında kontrol etmek.
- Ön koşullar: systemd 250 veya sonrası olan Linux. Root erişimi. Mümkünse iki çalıştırma: TPM2 olan bir bilgisayar ve TPM2 olmayan bir bilgisayar.
- Adımlar:
    1. docs/user/dev-service.md E.1 ve E.2 (Türkçe: T.1 ve T.2) ile kurun.
    2. `systemctl status vltd`.
    3. `journalctl -u vltd -b --no-pager`.
    4. `cat /etc/vltd/key-mode`.
    5. Hizmeti yeniden başlatın: `systemctl restart vltd`. 3. adımı tekrarlayın.
    6. Kullanıcınızı vlt grubuna ekleyin. vlt-dev-init.sh betiğini sudo ile başlattığınızda bunu kendisi yapar (SUDO_USER kullanıcısını ekler). Aksi halde: `sudo usermod -a -G vlt <kullanıcınız>`. Oturumu kapatıp yeniden açın. Sonra: `id -nG`.
- Beklenen sonuç: "active (running)". 6. adım: `id -nG` çıktısında `vlt` vardır. Günlükte `vault canary check passed state=available key_mode=host+tpm2` (TPM2 ile) veya `state="reduced protection" key_mode=host` (TPM2 olmadan), `audit chain verification ok=true`, `ipc server started` satırları vardır. UPS bağlı değilse: aygıt durumu "no device" olur ve vltd'yi durduran bir hata olmaz.
- Gönderin: 2. ve 4. adımların çıktısı, 3. ve 5. adımların günlük satırları (gerekirse bilgisayar adlarını silin), systemd sürümü (`systemctl --version`), TPM2 var mı yok mu ve 6. adımın sonucu (evet veya hayır; kullanıcı adını göndermeyin).

### T-03 Windows'ta vlt-setup ile vltd geliştirme kurulumu

- Amaç: vlt-setup aracını (TASK 03-00-00-002) gerçek bir Windows bilgisayarında kontrol etmek.
- Ön koşullar: Windows 10 veya 11 ya da Windows Server. Bir Administrator hesabı. SHA-256 kontrolü yapılmış `vltd-vlt-setup-windows-amd64` artifact dosyası.
- Adımlar:
    1. docs/user/dev-service.md E.3 (Türkçe: T.3) ile kurun. vlt-setup çıktısının tamamını kopyalayın.
    2. `Get-Service vltd`, `sc.exe qprivs vltd` ve `sc.exe qfailure vltd`.
    3. `icacls "$env:ProgramData\VoltioVolt"`.
    4. %ProgramData%\VoltioVolt\logs\vltd.log dosyasını açın.
    5. `.\dev-uninstall.ps1` ile kaldırın (veri klasörünü koruyun). Çıktıyı kopyalayın.
    6. Yeniden kurun (1. adım). vlt-setup, kendisini başlatan kullanıcıyı "VLT Operators" grubuna ekler ("1a." satırı). Oturumu kapatıp yeniden açın. Sonra: `net localgroup "VLT Operators"`.
- Beklenen sonuç: vlt-setup adımlarının hiçbiri hata bildirmez. 6. adım: kullanıcınız "VLT Operators" grubunun üyesidir. 5. adım: dev-uninstall grubu silmez, çünkü grubun bir üyesi vardır. Durum "Running". Gerekli ayrıcalıklar SeShutdownPrivilege ve SeChangeNotifyPrivilege. Kurtarma: restart, restart, none. ACL'de yalnızca SYSTEM, Administrators ve NT SERVICE\vltd vardır. Günlükte `vault canary check passed state=available key_mode=dpapi` vardır.
- Gönderin: 1, 2, 3 ve 5. adımların çıktısı, 4. adımın günlük satırları, Windows sürümü (`winver`) ve 6. adımın sonucu (evet veya hayır; kullanıcı adını göndermeyin).

### T-04 SEC-006 kopyalama testi

- Amaç: veri klasörünün bir kopyasının ikinci bir bilgisayarda Vault'u açmadığını kanıtlamak (VLT-RPT-003 7.5).
- Ön koşullar: A bilgisayarında T-02 veya T-03 tamamlanmıştır. Aynı işletim sistemine sahip ikinci bir B bilgisayarı.
- Adımlar: docs/user/dev-service.md E.6 (Türkçe: T.6).
- Beklenen sonuç: B'de Vault durumu "locked" olur (`vault locked reason="host binding failed"`). Durum "available" ise durun: bu bir güvenlik hatasıdır.
- Gönderin: B'nin Vault durumunu gösteren günlük satırları ve A ile B'nin işletim sistemi.

### T-05 Bir baytı değiştirilmiş Linux kimlik bilgisi

- Amaç: vltd-fallback.service birimini kontrol etmek (TASK 03-00-00-001 adım 3).
- Ön koşullar: T-02 tamamlanmıştır. Bir yedek alın: `cp /etc/credstore.encrypted/vlt-dek /root/vlt-dek.bak`.
- Adımlar:
    1. `systemctl stop vltd`.
    2. Dosyanın ortasındaki bir baytı değiştirin: `printf 'X' | dd of=/etc/credstore.encrypted/vlt-dek bs=1 seek=40 conv=notrunc`.
    3. `systemctl start vltd`. 10 saniye bekleyin.
    4. `systemctl status vltd vltd-fallback`.
    5. `journalctl -u vltd -u vltd-fallback -b --no-pager | tail -50`.
    6. Geri yükleyin: `systemctl stop vltd-fallback`, `cp /root/vlt-dek.bak /etc/credstore.encrypted/vlt-dek`, `systemctl reset-failed vltd`, `systemctl start vltd`.
- Beklenen sonuç: vltd.service "failed" (243/CREDENTIALS) olur. vltd-fallback.service "active (running)" olur. Günlükte `fallback mode: the primary unit failed` ve `vault locked reason="primary unit failed"` vardır. 6. adımdan sonra vltd "active (running)" ve Vault "available" olur.
- Gönderin: 4. ve 5. adımların çıktısı ve 6. adımdan sonraki durum.

### T-06 Yeniden başlatmalardan sonra Windows DPAPI canary

- Amaç: NT SERVICE\vltd sanal hesabı için kullanıcı kapsamlı DPAPI'yi kontrol etmek (VLT-RPT-003 7.5 madde 5; araştırmada UNVERIFIED).
- Ön koşullar: T-03 tamamlanmıştır (kaldırdıysanız yeniden kurun).
- Adımlar:
    1. `Restart-Service vltd`. 10 saniye bekleyin.
    2. vltd.log dosyasındaki son `vault canary check` satırını bulun.
    3. Bilgisayarı yeniden başlatın. `Start-Service vltd` (başlangıç türü manual). 2. adımı tekrarlayın.
- Beklenen sonuç: iki seferde de `vault canary check passed state=available key_mode=dpapi`.
- Gönderin: iki günlük satırı. Vault "locked" ise nedenini gösteren satırı da gönderin.

### T-07 polkit: vltd kullanıcısı olarak CanPowerOff ve CanHibernate

- Amaç: oturumu olmayan bir sistem hizmeti için logind'in hangi polkit eylem türünü kontrol ettiğini bulmak (VLT-RPT-003 7.4 madde 7).
- Ön koşullar: T-02 tamamlanmıştır. /etc/polkit-1/rules.d/50-vltd.rules polkit kuralı kuruludur.
- Adımlar (root olarak):
    1. `sudo -u vltd busctl call org.freedesktop.login1 /org/freedesktop/login1 org.freedesktop.login1.Manager CanPowerOff`
    2. `sudo -u vltd busctl call org.freedesktop.login1 /org/freedesktop/login1 org.freedesktop.login1.Manager CanHibernate`
    3. `busctl call org.freedesktop.login1 /org/freedesktop/login1 org.freedesktop.login1.Manager ListInhibitors`
    4. Bu testte PowerOff veya Hibernate çağırmayın.
- Beklenen sonuç: 1. adım `s "yes"` verir. 2. adım, hazırda bekletme yapılandırılmışsa `s "yes"`, değilse `"na"` veya `"no"` verir (o zaman vltd, hibernate eylemi için PowerOff kullanır).
- Gönderin: 1 ile 3. adımların çıktısı, dağıtım ve systemd sürümü.
