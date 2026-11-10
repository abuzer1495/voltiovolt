# Owner test plan / Owner test planı (VLT-IS-004 TASK 03-00-00-009, VLT-IS-005 TASK 03-00-00-009, VLT-IS-006 TASK 03-00-00-009)

English first. Turkish: section T below.

## E. English

Each test has: purpose, preconditions, steps, expected result, and what to send to the Instruction Author. Do the tests in order. Send the results of each test as soon as it is complete. Do not send a Secret, a PIN, a DEK, a QID value or a serial number. Check each file before you send it.

General preconditions for all tests:

- A test computer, not a production computer. Save all work before each test.
- The Reference UPS and its USB or serial cable.
- The CI artifacts of the latest successful `ci` run on `main`, with the SHA-256 check of docs/user/probe.md section 1.
- For T-08 to T-10: the VLT GUI artifact `vlt-gui-linux-amd64` or `vlt-gui-windows-amd64` with its SHA-256 file. Linux: the GTK 4 and WebKitGTK 6.0 libraries (for example on Debian and Ubuntu: `sudo apt install libgtk-4-1 libwebkitgtk-6.0-4`). Windows: the Microsoft Edge WebView2 Runtime (Windows 10 and 11 have it).
- Do the tests in this order: T-01 to T-09, then T-10 (OD-23), then T-11 and T-12 (VLT-IS-006).

### T-01 vlt-probe Capture on the Reference UPS

- Purpose: find the Dialect and the real responses of the Reference UPS (VLT-IS-002 TASK 03-00-00-008).
- Preconditions: vlt-probe from the CI artifact. The UPS is connected with USB. Use Linux if possible, because Windows has no raw report descriptor. The UPS battery is fully charged.
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

### T-08 Policy Simulation

- Purpose: check the Simulation of the default policy and of a changed policy (VLT-IS-004 TASK 03-00-00-006) with the VLT GUI.
- Preconditions: T-02 or T-03 is complete, vltd runs, and your user is in the group vlt (Linux) or "VLT Operators" (Windows). The VLT GUI artifact with the SHA-256 check. No UPS is necessary. Do not connect a UPS for this test: the Simulation does not turn off the computer, but an active policy can turn it off when a UPS reports a power failure.
- Steps:
    1. Start the VLT GUI. If no PIN is set, set a PIN. Unlock with the PIN.
    2. Open "Shutdown Policy". Write down the version and the state (active or inactive).
    3. Click "Run the Simulation". Write down the result of each scenario (passed or failed, and the time of the shutdown request or the cancel).
    4. Click "Activate". Read the confirmation text, then confirm. The state is "active".
    5. Click "Edit". Change "countdown_seconds" to 90. Click "Save".
    6. Check the text "The policy is now inactive. Run the Simulation to activate it." and the state "inactive". "Activate" is off.
    7. Click "Run the Simulation" again. Then "Activate" is on. Do not activate. Click "Edit", set "countdown_seconds" to 60 again and click "Save".
- Expected result: step 3: all scenarios "passed". Default policy: power_fail_long gives a shutdown request at 370 s, power_fail_short gives a cancel, low_battery gives a refused cancel and a request at 90 s, comm_lost_on_battery gives a request before its limit. Step 6: the policy is inactive after the change. Step 7: "Activate" is on only after the new Simulation passed.
- Send: the results of steps 2, 3, 6 and 7 (screenshots are permitted; check that no Secret is in them), the version of the VLT GUI (General, About) and the OS.

### T-09 Real shutdown with the Reference UPS

WARNING: This test turns off the computer. Save all work. Close all applications. Make sure that the UPS battery is fully charged. Set trigger.on_battery_seconds to 30. Disconnect the UPS input power. Connect the input power again after the computer is off.

- Purpose: check the full chain on real hardware: UPS, Device Manager, Event Engine, Policy Engine, OS shutdown executor (TASK 03-00-00-006 and 007).
- Preconditions: T-01 to T-03, T-07 and T-08 are complete. The computer is powered by the Reference UPS, and the UPS is connected to the computer with USB or serial. vltd runs (T-02 or T-03). The VLT GUI is the IPC client (OD-22). No other person uses the computer.
- Steps (all in the VLT GUI):
    1. Unlock with the PIN. Open "Shutdown Policy", click "Edit", set "trigger.on_battery_seconds" to 30 (keep the other defaults: countdown 60 s, action shutdown) and click "Save".
    2. Click "Run the Simulation". The result must be "passed". Then click "Activate" and confirm.
    3. Open "Device": the state must be "connected".
    4. Open "Dashboard". The mode is "Mains power". Write down the time.
    5. Disconnect the UPS input power. Write down the time.
    6. Watch the Dashboard: the mode changes to "On battery", the Event ON_BATTERY is at the top of the Events, after 30 seconds the countdown banner starts with 60 seconds.
    7. The computer turns off. Write down the time.
    8. Connect the UPS input power again. Start the computer.
    9. Linux: `journalctl -u vltd -b -1 --no-pager | tail -100`. Windows: vltd.log, and Event Viewer, System log, source User32, event ID 1074.
    10. Start the VLT GUI, unlock, open "Security" and read the audit log (policy_countdown_start, policy_shutdown_request, os_shutdown_request). Then set "trigger.on_battery_seconds" back to 300 on "Shutdown Policy".
- Expected result: the computer turns off about 90 seconds after step 5, without a dialog. The audit log has the three entries. Linux: os_shutdown_request has the inhibitor list. Windows: event 1074 shows the reason code 0x6000c ("Power Failure: Environment"; the exact event text is UNVERIFIED).
- Abort: to stop the countdown, connect the input power again before the end (cancel_when_mains_returns is on), or click "Cancel" in the countdown banner (not possible with LOW_BATTERY).
- Send: the times of steps 4, 5 and 7, the log lines, the audit entries (no Secret is in them), and the Windows event 1074 text.

### T-10 VLT GUI test

- Purpose: check the VLT GUI (VLT-IS-005) on the Owner computer, and review all Turkish text (VLT-IS-005 TASK 03-00-00-008 step 1.4).
- Preconditions: T-01 to T-09 are complete (OD-23). vltd runs, and your user is in the group vlt (Linux) or "VLT Operators" (Windows). The VLT GUI artifact with the SHA-256 check. For a new first start: a new vltd data directory without a PIN (optional).
- Steps:
    1. Connect: start the VLT GUI. The top bar shows "Connected to vltd". Stop vltd for 20 seconds, then start it again: the GUI shows "No connection to vltd" with the reason, and connects again in 5 seconds or less after vltd runs.
    2. Set PIN (only on a new data directory): the GUI shows only "Set PIN" with the PIN rules. Try a PIN with 5 characters (refused), then set a correct PIN.
    3. Login: click "Unlock" and enter a wrong PIN until the GUI shows the block time. Wait, then unlock with the correct PIN.
    4. Lock: click "Lock". The GUI shows "Locked" and still shows the Dashboard. Unlock again. Then wait for the idle time (auth.session_idle_minutes) without a click: the GUI locks itself.
    5. Dashboard values: compare the values (input and output voltage, frequency, load, battery charge and voltage, runtime, temperature) with the UPS display or the UPS documentation.
    6. Device screen: the state "connected", the Transport, the Dialect, the model data. The serial number is not shown in a report.
    7. History charts: open each range (1 hour to 1 year) and each chart. Open the table view of one chart.
    8. Events: load older Events, filter by severity and by type.
    9. Policy edit: change one value outside its range (for example countdown_seconds 5): the GUI refuses it before it sends it. Change it to a correct value and save.
    10. Simulation: run it and check that each scenario shows its steps, its virtual times and "passed" or "failed".
    11. Activation: "Activate" is on only after a passed Simulation. Activate, then deactivate with the confirmation.
    12. Language change: open "General", select "Türkçe", then "English". Close and start the VLT GUI again: the language stays.
    13. Review of all Turkish text: select "Türkçe" and open each screen and each message that you can see. Check the Turkish characters (ç, ğ, ı, İ, ö, ş, ü), the words and the grammar.
    14. Keyboard: use only the Tab, Shift+Tab, Enter and Space keys on one screen. The focus is always visible.
    15. Theme: change the OS theme between light and dark. The GUI follows it.
- Expected result: all steps work as written. No text in the GUI is in the wrong language or has wrong Turkish characters.
- Send: the result of each step (OK or the problem), a list of the Turkish text that must change (the screen, the current text and the correct text), the OS, and the version of the VLT GUI (General, About). Screenshots are permitted; check that no Secret, no PIN and no serial number is in them.

### T-11 UPS Control (VLT-IS-006)

- Purpose: check the UPS Control screen and the CONTROL commands test and beeper on the Reference UPS (VLT-IS-006 TASK 03-00-00-005, 007 and 008).
- Preconditions: T-01 to T-10 are complete. vltd runs with `device.transport` serial and the port of T-01 (screen "Device"). The device state is "connected". Mains power is present. The UPS battery is fully charged.
- Steps (all in the VLT GUI):
    1. Unlock with the PIN. Open "UPS Control". The buttons are on.
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
- Preconditions: T-09 and T-11 are complete. Only the test computer is connected to the UPS output. vltd runs with the port of T-01.
- Steps (all in the VLT GUI):
    1. Unlock with the PIN. Open "Shutdown Policy", click "Edit", set ups_off.enabled on, ups_off.delay_minutes 2, ups_off.restore_minutes 1 (keep trigger.on_battery_seconds 30 of T-09) and click "Save".
    2. Run the Simulation. The result must be "passed" and each scenario shows the step "UPS output off command (dry run, not sent)". Activate the policy.
    3. Disconnect the UPS input power. Write down the time.
    4. Record the times of the OS shutdown, the UPS output off and the UPS output on.
    5. Run 1: connect the input power again before the restore time (before the UPS output is on again).
    6. Run 2: do steps 3 and 4 again, and do not connect the input power. Write down what the UPS does. Then connect the input power.
    7. Start the computer. Open "Security" and read the audit log: policy_countdown_start, policy_ups_off, policy_shutdown_request, os_shutdown_request.
    8. Set ups_off.enabled off again (the default, OD-28), run the Simulation and activate the policy.
- Expected result: the OS shuts down before the UPS output turns off (2 minutes after the shutdown starts). Run 1: the UPS output turns on 1 minute after it turned off. Run 2: the behavior without input power is UNVERIFIED (VLT-REF-001 5.1 item 4); write it down.
- Send: the times of steps 3 and 4 for both runs, the audit entries and the vltd log lines.

## T. Türkçe

Her testte şunlar vardır: amaç, ön koşullar, adımlar, beklenen sonuç ve Instruction Author'a gönderilecekler. Testleri sırayla yapın. Her testin sonuçlarını test biter bitmez gönderin. Secret, PIN, DEK, QID değeri veya seri numarası göndermeyin. Göndermeden önce her dosyayı kontrol edin.

Tüm testler için genel ön koşullar:

- Üretim bilgisayarı değil, bir test bilgisayarı. Her testten önce tüm çalışmalarınızı kaydedin.
- Referans UPS ve USB veya seri kablosu.
- `main` dalındaki son başarılı `ci` çalışmasının CI artifact dosyaları; SHA-256 kontrolü docs/user/probe.md bölüm 1'e göre yapılmış olmalıdır.
- T-08 ile T-10 için: SHA-256 dosyasıyla birlikte `vlt-gui-linux-amd64` veya `vlt-gui-windows-amd64` VLT GUI artifact dosyası. Linux: GTK 4 ve WebKitGTK 6.0 kitaplıkları (örneğin Debian ve Ubuntu'da: `sudo apt install libgtk-4-1 libwebkitgtk-6.0-4`). Windows: Microsoft Edge WebView2 Runtime (Windows 10 ve 11'de vardır).
- Testleri şu sırayla yapın: T-01 ile T-09, sonra T-10 (OD-23), sonra T-11 ve T-12 (VLT-IS-006).

### T-01 Referans UPS ile vlt-probe Capture

- Amaç: Referans UPS'in Dialect'ini ve gerçek yanıtlarını bulmak (VLT-IS-002 TASK 03-00-00-008).
- Ön koşullar: CI artifact dosyasından vlt-probe. UPS, USB ile bağlıdır. Mümkünse Linux kullanın, çünkü Windows'ta ham rapor tanımlayıcısı (raw report descriptor) yoktur. UPS bataryası tam doludur.
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

### T-08 Politika Simülasyonu

- Amaç: varsayılan politikanın ve değiştirilmiş bir politikanın Simülasyonunu (VLT-IS-004 TASK 03-00-00-006) VLT GUI ile kontrol etmek.
- Ön koşullar: T-02 veya T-03 tamamlanmıştır, vltd çalışır ve kullanıcınız vlt (Linux) veya "VLT Operators" (Windows) grubundadır. SHA-256 kontrolü yapılmış VLT GUI artifact dosyası. UPS gerekmez. Bu test için UPS bağlamayın: Simülasyon bilgisayarı kapatmaz, ancak etkin bir politika, UPS bir elektrik kesintisi bildirdiğinde bilgisayarı kapatabilir.
- Adımlar:
    1. VLT GUI'yi başlatın. PIN belirlenmemişse bir PIN belirleyin. PIN ile kilidi açın.
    2. "Kapatma Politikası" ekranını açın. Sürümü ve durumu (etkin veya etkin değil) not edin.
    3. "Simülasyonu çalıştır" düğmesine tıklayın. Her senaryonun sonucunu not edin (başarılı veya başarısız ve kapatma isteğinin veya iptalin zamanı).
    4. "Etkinleştir" düğmesine tıklayın. Onay metnini okuyun ve onaylayın. Durum "etkin" olur.
    5. "Düzenle" düğmesine tıklayın. "countdown_seconds" değerini 90 yapın. "Kaydet" düğmesine tıklayın.
    6. "Politika artık etkin değil. Etkinleştirmek için Simülasyonu çalıştırın." metnini ve "etkin değil" durumunu kontrol edin. "Etkinleştir" kapalıdır.
    7. Yeniden "Simülasyonu çalıştır" düğmesine tıklayın. Sonra "Etkinleştir" açıktır. Etkinleştirmeyin. "Düzenle" düğmesine tıklayın, "countdown_seconds" değerini yeniden 60 yapın ve "Kaydet" düğmesine tıklayın.
- Beklenen sonuç: 3. adım: tüm senaryolar "başarılı". Varsayılan politika: power_fail_long 370. saniyede bir kapatma isteği verir, power_fail_short bir iptal verir, low_battery reddedilen bir iptal ve 90. saniyede bir istek verir, comm_lost_on_battery sınırından önce bir istek verir. 6. adım: değişiklikten sonra politika etkin değildir. 7. adım: "Etkinleştir" yalnızca yeni Simülasyon başarılı olduktan sonra açıktır.
- Gönderin: 2, 3, 6 ve 7. adımların sonuçları (ekran görüntüsü gönderilebilir; içlerinde Secret olmadığını kontrol edin), VLT GUI sürümü (Genel, Hakkında) ve işletim sistemi.

### T-09 Referans UPS ile gerçek kapatma

UYARI: Bu test bilgisayarı kapatır. Tüm çalışmalarınızı kaydedin. Tüm uygulamaları kapatın. UPS bataryasının tam dolu olduğundan emin olun. trigger.on_battery_seconds değerini 30 yapın. UPS giriş gücünü kesin. Bilgisayar kapandıktan sonra giriş gücünü yeniden bağlayın.

(English, verbatim: "WARNING: This test turns off the computer. Save all work. Close all applications. Make sure that the UPS battery is fully charged. Set trigger.on_battery_seconds to 30. Disconnect the UPS input power. Connect the input power again after the computer is off.")

- Amaç: tüm zinciri gerçek donanımda kontrol etmek: UPS, Device Manager, Event Engine, Policy Engine, işletim sistemi kapatma yürütücüsü (TASK 03-00-00-006 ve 007).
- Ön koşullar: T-01 ile T-03, T-07 ve T-08 tamamlanmıştır. Bilgisayar Referans UPS'ten beslenir ve UPS bilgisayara USB veya seri kablo ile bağlıdır. vltd çalışır (T-02 veya T-03). IPC istemcisi VLT GUI'dir (OD-22). Bilgisayarı başka kimse kullanmaz.
- Adımlar (tümü VLT GUI'de):
    1. PIN ile kilidi açın. "Kapatma Politikası" ekranını açın, "Düzenle" düğmesine tıklayın, "trigger.on_battery_seconds" değerini 30 yapın (diğer varsayılanları koruyun: geri sayım 60 sn, eylem kapatma) ve "Kaydet" düğmesine tıklayın.
    2. "Simülasyonu çalıştır" düğmesine tıklayın. Sonuç "başarılı" olmalıdır. Sonra "Etkinleştir" düğmesine tıklayın ve onaylayın.
    3. "Cihaz" ekranını açın: durum "bağlı" olmalıdır.
    4. "Gösterge paneli" ekranını açın. Mod "Şebeke elektriği" olur. Zamanı not edin.
    5. UPS giriş gücünü kesin. Zamanı not edin.
    6. Gösterge panelini izleyin: mod "Aküde" olur, ON_BATTERY Olayı Olayların en üstündedir, 30 saniye sonra 60 saniyelik geri sayım şeridi başlar.
    7. Bilgisayar kapanır. Zamanı not edin.
    8. UPS giriş gücünü yeniden bağlayın. Bilgisayarı başlatın.
    9. Linux: `journalctl -u vltd -b -1 --no-pager | tail -100`. Windows: vltd.log ve Olay Görüntüleyicisi, Sistem günlüğü, kaynak User32, olay kimliği 1074.
    10. VLT GUI'yi başlatın, kilidi açın, "Güvenlik" ekranını açın ve denetim kaydını okuyun (policy_countdown_start, policy_shutdown_request, os_shutdown_request). Sonra "Kapatma Politikası" ekranında "trigger.on_battery_seconds" değerini yeniden 300 yapın.
- Beklenen sonuç: bilgisayar 5. adımdan yaklaşık 90 saniye sonra, bir iletişim kutusu olmadan kapanır. Denetim günlüğünde üç kayıt vardır. Linux: os_shutdown_request kaydında engelleyici (inhibitor) listesi vardır. Windows: 1074 olayı 0x6000c neden kodunu gösterir ("Power Failure: Environment"; olay metninin tam hali UNVERIFIED).
- Durdurma: geri sayımı durdurmak için, süre bitmeden giriş gücünü yeniden bağlayın (cancel_when_mains_returns açıktır) veya geri sayım şeridindeki "İptal" düğmesine tıklayın (LOW_BATTERY etkinken mümkün değildir).
- Gönderin: 4, 5 ve 7. adımların zamanları, günlük satırları, denetim kayıtları (içlerinde Secret yoktur) ve Windows 1074 olay metni.

### T-10 VLT GUI testi

- Amaç: VLT GUI'yi (VLT-IS-005) Owner bilgisayarında kontrol etmek ve tüm Türkçe metinleri gözden geçirmek (VLT-IS-005 TASK 03-00-00-008 adım 1.4).
- Ön koşullar: T-01 ile T-09 tamamlanmıştır (OD-23). vltd çalışır ve kullanıcınız vlt (Linux) veya "VLT Operators" (Windows) grubundadır. SHA-256 kontrolü yapılmış VLT GUI artifact dosyası. Yeni bir ilk başlatma için: PIN'i olmayan yeni bir vltd veri klasörü (isteğe bağlı).
- Adımlar:
    1. Bağlanma: VLT GUI'yi başlatın. Üst çubukta "vltd'ye bağlı" yazar. vltd'yi 20 saniye durdurun, sonra yeniden başlatın: GUI nedeniyle birlikte "vltd ile bağlantı yok" gösterir ve vltd çalıştıktan sonra en geç 5 saniye içinde yeniden bağlanır.
    2. PIN belirleme (yalnızca yeni bir veri klasöründe): GUI yalnızca PIN kurallarıyla birlikte "PIN belirle" ekranını gösterir. 5 karakterli bir PIN deneyin (reddedilir), sonra doğru bir PIN belirleyin.
    3. Giriş: "Kilidi aç" düğmesine tıklayın ve GUI bekleme süresini gösterene kadar yanlış PIN girin. Bekleyin, sonra doğru PIN ile kilidi açın.
    4. Kilitleme: "Kilitle" düğmesine tıklayın. GUI "Kilitli" gösterir ve Gösterge panelini göstermeye devam eder. Kilidi yeniden açın. Sonra hiçbir yere tıklamadan boşta kalma süresini (auth.session_idle_minutes) bekleyin: GUI kendini kilitler.
    5. Gösterge paneli değerleri: değerleri (giriş ve çıkış gerilimi, frekans, yük, akü doluluğu ve gerilimi, çalışma süresi, sıcaklık) UPS ekranıyla veya UPS belgeleriyle karşılaştırın.
    6. Cihaz ekranı: "bağlı" durumu, bağlantı türü, Diyalekt, model bilgileri. Seri numarasını raporda göndermeyin.
    7. Geçmiş grafikleri: her aralığı (1 saatten 1 yıla) ve her grafiği açın. Bir grafiğin tablo görünümünü açın.
    8. Olaylar: daha eski Olayları yükleyin, önem derecesine ve türe göre filtreleyin.
    9. Politika düzenleme: bir değeri izin verilen aralığın dışına çıkarın (örneğin countdown_seconds 5): GUI bunu göndermeden önce reddeder. Doğru bir değer girin ve kaydedin.
    10. Simülasyon: çalıştırın ve her senaryonun adımlarını, sanal zamanlarını ve "başarılı" veya "başarısız" sonucunu gösterdiğini kontrol edin.
    11. Etkinleştirme: "Etkinleştir" yalnızca başarılı bir Simülasyondan sonra açıktır. Etkinleştirin, sonra onay ile devre dışı bırakın.
    12. Dil değişikliği: "Genel" ekranını açın, "English" seçin, sonra "Türkçe" seçin. VLT GUI'yi kapatıp yeniden başlatın: dil korunur.
    13. Tüm Türkçe metinlerin gözden geçirilmesi: "Türkçe" seçin ve her ekranı ve görebildiğiniz her mesajı açın. Türkçe karakterleri (ç, ğ, ı, İ, ö, ş, ü), sözcükleri ve dil bilgisini kontrol edin.
    14. Klavye: bir ekranda yalnızca Tab, Shift+Tab, Enter ve Boşluk tuşlarını kullanın. Odak her zaman görünür.
    15. Tema: işletim sisteminin temasını açık ve koyu arasında değiştirin. GUI temayı izler.
- Beklenen sonuç: tüm adımlar yazıldığı gibi çalışır. GUI'deki hiçbir metin yanlış dilde değildir veya yanlış Türkçe karakter içermez.
- Gönderin: her adımın sonucu (Tamam veya sorun), değişmesi gereken Türkçe metinlerin listesi (ekran, geçerli metin ve doğru metin), işletim sistemi ve VLT GUI sürümü (Genel, Hakkında). Ekran görüntüsü gönderilebilir; içlerinde Secret, PIN ve seri numarası olmadığını kontrol edin.

### T-11 UPS Kontrolü (VLT-IS-006)

- Amaç: UPS Kontrolü ekranını ve Referans UPS üzerinde test ve sesli uyarı CONTROL komutlarını kontrol etmek (VLT-IS-006 TASK 03-00-00-005, 007 ve 008).
- Ön koşullar: T-01 ile T-10 tamamlanmıştır. vltd, `device.transport` serial ve T-01'deki port ile çalışır ("Cihaz" ekranı). Cihaz durumu "bağlı"dır. Şebeke gücü vardır. UPS bataryası tam doludur.
- Adımlar (tümü VLT GUI'de):
    1. PIN ile kilidi açın. "UPS Kontrolü" ekranını açın. Düğmeler açıktır.
    2. "Sesli uyarı kapalı" düğmesine tıklayın. Zamanı not edin. UPS giriş gücünü 10 saniye kesin: UPS ses çıkarmaz. Giriş gücünü yeniden bağlayın.
    3. "Sesli uyarı açık" düğmesine tıklayın. UPS giriş gücünü 10 saniye kesin: UPS ses çıkarır. Giriş gücünü yeniden bağlayın.
    4. "10 saniye test" düğmesine tıklayın, sonra "Testi başlat" ile onaylayın. Zamanı not edin. TEST_STARTED Olayı görünür. Testten sonra ekran son test sonucunu (TEST_PASSED veya TEST_FAILED) zamanıyla gösterir.
    5. "Olaylar" ekranını açın: TEST_STARTED ve bitişi.
    6. "Güvenlik" ekranını açın ve denetim kaydını okuyun: ups_beeper_set, ups_control, ups_control_verify, ups_test_start ve ups_test_end.
- Beklenen sonuç: her komut "doğrulandı" durumundadır. Test sonucu TEST_PASSED'dır. Denetim kaydında 6. adımdaki girdiler baytlarıyla (Q, T) vardır.
- Gönderin: zamanlar, test sonucu, denetim girdileri (içlerinde Secret yoktur) ve Olaylar.

### T-12 UPS çıkışını kapatma ve yeniden açma (VLT-IS-006)

UYARI: Bu test sırasında UPS çıkışını kapatır. UPS'e bağlı tüm cihazların gücü kesilir. UPS'e yalnızca test bilgisayarını bağlayın. Tüm çalışmalarınızı kaydedin. Bataryanın tam dolu olduğundan emin olun.

- Amaç: Kapatma Politikasının UPS çıkışını kapatma adımını kontrol etmek (VLT-IS-006 TASK 03-00-00-006, OD-28).
- Ön koşullar: T-09 ve T-11 tamamlanmıştır. UPS çıkışına yalnızca test bilgisayarı bağlıdır. vltd, T-01'deki port ile çalışır.
- Adımlar (tümü VLT GUI'de):
    1. PIN ile kilidi açın. "Kapatma Politikası" ekranını açın, "Düzenle" düğmesine tıklayın, ups_off.enabled açık, ups_off.delay_minutes 2, ups_off.restore_minutes 1 yapın (T-09'daki trigger.on_battery_seconds 30 kalsın) ve "Kaydet" düğmesine tıklayın.
    2. Simülasyonu çalıştırın. Sonuç "başarılı" olmalıdır ve her senaryo "UPS çıkışını kapatma komutu (deneme, gönderilmedi)" adımını gösterir. Politikayı etkinleştirin.
    3. UPS giriş gücünü kesin. Zamanı not edin.
    4. İşletim sisteminin kapanma, UPS çıkışının kapanma ve UPS çıkışının açılma zamanlarını kaydedin.
    5. 1. çalıştırma: giriş gücünü yeniden açma süresinden önce (UPS çıkışı yeniden açılmadan önce) bağlayın.
    6. 2. çalıştırma: 3. ve 4. adımları tekrarlayın ve giriş gücünü bağlamayın. UPS'in ne yaptığını not edin. Sonra giriş gücünü bağlayın.
    7. Bilgisayarı başlatın. "Güvenlik" ekranını açın ve denetim kaydını okuyun: policy_countdown_start, policy_ups_off, policy_shutdown_request, os_shutdown_request.
    8. ups_off.enabled ayarını yeniden kapatın (varsayılan, OD-28), Simülasyonu çalıştırın ve politikayı etkinleştirin.
- Beklenen sonuç: işletim sistemi, UPS çıkışı kapanmadan önce (kapatma başladıktan 2 dakika sonra) kapanır. 1. çalıştırma: UPS çıkışı kapandıktan 1 dakika sonra açılır. 2. çalıştırma: giriş gücü olmadan davranış UNVERIFIED'dır (VLT-REF-001 5.1 madde 4); not edin.
- Gönderin: her iki çalıştırma için 3. ve 4. adımların zamanları, denetim girdileri ve vltd günlük satırları.
