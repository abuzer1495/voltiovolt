# VoltioVolt user guide

This guide is for the portable application VoltioVolt (test builds, tag `test-latest`). The Help menu item "Documentation" ("Belgeler") and the F1 key open this folder. The Turkish text of a screen element is in brackets, for example "Device" ("Cihaz").

Other guides in this folder:

- [simulator.md](simulator.md): the UPS simulator vlt-simulator.
- [probe.md](probe.md): the capture tool vlt-probe.
- [owner-tests.md](owner-tests.md): the Owner test plan.
- [dev-service.md](dev-service.md): service mode (vltd), for development only.

WARNING: VoltioVolt can shut down your computer. Use a test computer for the test builds. Save your work before a test.

## 1. Download

1. Open https://github.com/abuzer1495/voltiovolt/releases and select the pre-release "Test build" (tag `test-latest`). CI makes it again after each change on `main`. There is no other release.
2. Download the package for your system and `SHA256SUMS.txt`:

| System | Package | Contents |
|--------|---------|----------|
| Windows 10 and 11, x64 | `VoltioVolt-test-windows-amd64.zip` | VoltioVolt.exe, vlt-probe.exe, vlt-simulator.exe, install.cmd, uninstall.cmd, remove-old-test-service.cmd, OKUBENI-README.txt |
| Linux, x86-64 | `VoltioVolt-test-linux-amd64.tar.gz` | voltiovolt, vlt-probe, vlt-simulator, install.sh, uninstall.sh, OKUBENI-README.txt |

3. Check the SHA-256. If the value is not the same, do not run the program.
   - Windows (PowerShell): `Get-FileHash VoltioVolt-test-windows-amd64.zip -Algorithm SHA256`. Compare the value with the line in `SHA256SUMS.txt`.
   - Linux: `sha256sum --ignore-missing -c SHA256SUMS.txt`. The result must be `OK`.

The files are not signed.

## 2. Install and start

VoltioVolt is portable. It needs no installation and no administrator rights, and it does not change the operating system: no service, no driver, no firewall rule, no registry entry for all users. The only exception is the optional switch "Start at logon" ("Oturum açılışında başlat") on the General screen (default: off).

### Windows

1. Method 1: extract all files of the zip file to a folder. Double-click `VoltioVolt.exe`.
2. Method 2: run `install.cmd`. It copies the files to `%LOCALAPPDATA%\Programs\VoltioVolt` and starts VoltioVolt. It makes a Start menu shortcut only if you answer Y.
3. SmartScreen: at "Windows protected your PC", select "More info", then "Run anyway".
4. The Microsoft Edge WebView2 Runtime is necessary. Windows 10 and 11 have it.
5. If the old VLT-IS-007 test service is installed, run `remove-old-test-service.cmd` one time. Only this script needs administrator rights.

### Linux

1. Run-time packages (Debian, Ubuntu): `sudo apt install libgtk-4-1 libwebkitgtk-6.0-4`.
2. Method 1: `tar xzf VoltioVolt-test-linux-amd64.tar.gz`, `cd VoltioVolt-test-linux-amd64`, `./voltiovolt`.
3. Method 2: `sh install.sh` (do not use sudo). It copies the files to `~/.local/opt/voltiovolt`. It makes `~/.local/share/applications/voltiovolt.desktop` only if you agree.
4. Serial port access: if your user cannot open `/dev/ttyUSB*` or `/dev/ttyACM*`, run `sudo usermod -aG dialout <user name>` one time, then log out and log in. VoltioVolt shows this command but does not run it.

### Before the first start

Stop other UPS programs, for example UPSilon 2000 and the RupsMon service. Only one program can use the UPS port at a time. Do not run vlt-probe on the same port while VoltioVolt runs.

### One process, tray

Only one VoltioVolt runs for each user. A second start shows the window of the first one. When you close the window, VoltioVolt continues to monitor the UPS in the tray. To stop it, select "Quit" ("Çık") in the tray menu, or File > "Exit" ("Çıkış"), and confirm. If you quit, VoltioVolt stops the monitoring and the automatic shutdown.

## 3. Setup wizard

At the first start the setup wizard opens (window size 760 × 560). The steps are:

1. PIN ("PIN"): optional, see section 4.
2. UPS port ("UPS portu"): select the COM port (Windows, for example `COM3`) or the port (Linux, for example `/dev/ttyUSB0`), or USB HID. You can also select "Demo (simulator)" or "Test connection (simulator)".
3. Live values ("Canlı değerler"): check that the values from the UPS are correct.
4. Shutdown Policy ("Kapatma Politikası"): the triggers, the countdown and the Simulation. This step also checks that your user can shut down the computer.
5. Telegram and 6. E-mail ("E-posta"): optional. You can Skip these steps.
7. Modem and router ("Modem ve yönlendirici"): optional.
8. Interface ("Arayüz"): the interface size and the interface mode.
9. Summary ("Özet").

To run the wizard again, open the General screen and select "Start the setup wizard" ("Kurulum sihirbazını başlat").

## 4. PIN

A PIN is optional. A PIN prevents other people at this computer from changing the shutdown settings.

1. Without a PIN, all functions are available. Secret fields (the Telegram bot token, the SMTP password) are always write-only: after you save a value, VoltioVolt shows only a hint.
2. Set PIN ("PIN belirle") and Remove PIN ("PIN kaldır") are on the General screen. Remove PIN needs the current PIN.
3. With a PIN, the window starts locked. A locked window still shows the Dashboard. Settings, Secrets, CONTROL commands and the activation of a Shutdown Policy need the unlocked state.
4. Session: after you unlock, the session stays open while you use the window. Only changes and your keyboard and mouse input count as use; the automatic updates of the values do not. After 10 minutes without use (setting `auth.session_idle_minutes`, 1 to 120) or after 8 hours, VoltioVolt locks again. The "Lock" ("Kilitle") button locks at once. In the wizard, a step that needs the PIN asks for it inside the wizard.
5. Forgot PIN ("PIN'i unuttum"): on the lock screen, select "Forgot PIN", then type `RESET` and select "Reset PIN" ("PIN'i sıfırla"). This removes the PIN and deletes all saved passwords and tokens (Telegram, e-mail). The automatic shutdown becomes inactive. Your settings and your history stay. Set the Secrets and activate the Shutdown Policy again after a reset.

## 5. Interface

### Simple mode and Expert mode

- Simple mode ("Basit kip") is the default. It shows one screen with the health of the UPS, the automatic shutdown state, the input voltage and load of the last hour, the last events and a button to the settings.
- Expert mode ("Uzman kipi") shows all screens: Dashboard, Device, UPS Control, History, Events, Shutdown Policy, Notifications, Security, General.
- Switch with the View menu ("Görünüm"), the "Expert mode" / "Simple mode" button, or in the wizard. The choice is saved.

### Interface size

- The interface size ("Arayüz boyutu") is Auto ("Otomatik") or 80 %, 90 %, 100 %, 110 %, 125 %, 150 %, 175 % or 200 %. Auto follows the display scale of the operating system.
- Change it in the View menu, on the General screen or in the wizard. Keys: Ctrl+Plus (larger), Ctrl+Minus (smaller), Ctrl+0 (Auto).
- If a size does not fit on the screen, VoltioVolt uses the largest size that fits and tells you.

### Look

The window has a compact Windows 11 style on all systems: a menu bar (File / Dosya, View / Görünüm, Tools / Araçlar, Help / Yardım), a navigation pane with icons that you can collapse, and light and dark colors that follow the operating system. The language is Turkish or English (General screen).

## 6. Help

- Each field has a one-sentence help text. A "?" button opens more help and, where useful, a link to an official help page. VoltioVolt opens only official pages (core.telegram.org, support.google.com, support.microsoft.com), and always in your web browser.
- The Help menu ("Yardım") has "Documentation" (this guide, also F1) and "About VoltioVolt".

## 7. Demo mode

Demo mode lets you see VoltioVolt work without a UPS.

1. On the Device screen (or in the wizard), select "Demo (simulator)" ("Demo (simülatör)"). An in-process simulator gives live values. The banner "DEMO" is shown.
2. "Simulate power failure" ("Güç kesintisini simüle et") puts the simulated UPS on battery for 60 seconds.
3. A shutdown in Demo mode is a dry run. VoltioVolt shows "Demo: the computer would shut down now." and does not shut down the computer.
4. "Leave demo" ("Demodan çık") returns to the real port.
5. Demo data has the source "demo". History and Events hide it unless you select "Show other sources" ("Diğer kaynakları göster").

## 8. Test connection with vlt-simulator

vlt-simulator is a UPS simulator in a terminal window. It is in the package. It connects only through a local named pipe (Windows) or a Unix socket (Linux) of your user. It has no network connection and does not use a real port.

1. Start `vlt-simulator.exe` (Windows) or `./vlt-simulator` (Linux).
2. In VoltioVolt, open Device and select "Test connection (simulator)" ("Test bağlantısı (simülatör)"). The banner "TEST CONNECTION" ("TEST BAĞLANTISI") is shown.
3. Change the values with the arrow keys, or press 1 to 9 for a scenario (for example a power failure or a low battery). VoltioVolt shows the changes at once.
4. Data of the test connection has the source "test". History and Events hide it unless you select "Show other sources".
5. A shutdown through the test connection is a dry run: VoltioVolt shows "Test: the computer would shut down now." The setting "Permit a real computer shutdown in a test" (`test.real_shutdown`) is off by default. Keep it off. With this setting on, a test can turn off this computer.

[simulator.md](simulator.md) has all keys, menu items and scenarios, and the option `--headless` for CI.

## 9. History

- The History screen ("Geçmiş") has charts for 1 hour, 24 hours, 7 days, 30 days and 1 year: voltages, load, battery voltage and charge, frequency and temperature.
- Live ("Canlı"): the charts update with each new value. "Pause" ("Duraklat") stops the update, "Resume" ("Sürdür") starts it again.
- Series ("Seriler"): select which lines each chart shows. The choice is saved.
- "Show the values as a table" gives the values of a chart as text.

## 10. Shutdown Policy and Notifications

1. Shutdown Policy: set the triggers (time on battery, charge, runtime, low battery flag, communication lost on battery) and the countdown. Run the Simulation ("Simülasyonu çalıştır"). Activate ("Etkinleştir") is possible only after a passed Simulation of the current version. During a countdown you can cancel the shutdown.
2. Windows: a normal user can shut down the computer (VoltioVolt enables the shutdown privilege of your user). Linux: VoltioVolt asks logind on the system D-Bus. If your user cannot shut down the computer, the Simulation shows "This user cannot shut down the computer." and the policy cannot be activated.
3. Notifications ("Bildirimler"): Desktop, Telegram (bot token from @BotFather, then pair a chat with `/start` and the 6-digit code) and E-mail (SMTP; for Gmail use an app password). Each channel has a minimum severity and "Send test message".

## 11. Data folder and Vault

1. The data folder holds the database, the Vault, the logs and the audit log. The General screen shows its path ("Veri klasörü").
2. Default: the folder `data` next to the program, if it is writable. Else: Windows `%LOCALAPPDATA%\VoltioVolt`, Linux `~/.local/share/voltiovolt`.
3. Make a copy of this folder to keep your history.
4. The Vault keeps the Secrets encrypted:
   - Windows: DPAPI of the current user.
   - Linux: the Secret Service (for example GNOME Keyring or KWallet) if it is available. Else a key file `vault.dek` with mode 0600 in the data folder; the Security screen then shows "reduced protection".
5. With DPAPI or the Secret Service, a copy of the data folder on another computer or for another user cannot open the Vault. Set the Secrets again there. With the key file, the key is in the folder: keep a copy of the data folder private.

## 12. Uninstall

1. Quit VoltioVolt (tray menu, "Quit").
2. Windows: run `uninstall.cmd`. Linux: run `sh uninstall.sh`. The scripts remove the program files, the Start menu or desktop shortcut and the start-at-logon shortcut. They ask separately before they delete the data folder.
3. Method 1 (no install): delete the folder. The data folder is inside it, unless VoltioVolt used the profile folder (section 11).
4. Method 1 only: if "Start at logon" was on, switch it off before you delete the folder, or delete the shortcut: Windows `shell:startup`, Linux `~/.config/autostart/voltiovolt.desktop`.
