# Owner test plan / Owner test planı (VLT-IS-004 TASK 03-00-00-009)

English first. Turkish: section T below.

## E. English

Each test has: purpose, preconditions, steps, expected result, and what to send to the Instruction Author. Do the tests in order. Send the results of each test as soon as it is complete. Do not send a Secret, a PIN, a DEK, a QID value or a serial number. Check each file before you send it.

General preconditions for all tests:

- A test computer, not a production computer. Save all work before each test.
- The Reference UPS and its USB or serial cable.
- The CI artifacts of the latest successful `ci` run on `main`, with the SHA-256 check of docs/user/probe.md section 1.

### T-01 vlt-probe Capture on the Reference UPS

- Purpose: find the Dialect and the real responses of the Reference UPS (VLT-IS-002 TASK 03-00-00-008).
- Preconditions: vlt-probe from the CI artifact. The UPS is connected with USB. Use Linux if possible, because Windows has no raw report descriptor. The UPS battery is fully charged.
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
- Expected result: "active (running)". The log has `vault canary check passed state=available key_mode=host+tpm2` (with TPM2) or `state="reduced protection" key_mode=host` (without TPM2), `audit chain verification ok=true`, `ipc server started`. If no UPS is connected: device state "no device" and no error that stops vltd.
- Send: the output of step 2 and step 4, the log lines of step 3 and step 5 (remove host names if necessary), the systemd version (`systemctl --version`), TPM2 yes or no.

### T-03 vltd development install on Windows with vlt-setup

- Purpose: check vlt-setup (TASK 03-00-00-002) on a real Windows computer.
- Preconditions: Windows 10 or 11, or Windows Server. An Administrator account. The artifact `vltd-vlt-setup-windows-amd64` with the SHA-256 check.
- Steps:
    1. Install with docs/user/dev-service.md E.3. Copy all output of vlt-setup.
    2. `Get-Service vltd` and `sc.exe qprivs vltd` and `sc.exe qfailure vltd`.
    3. `icacls "$env:ProgramData\VoltioVolt"`.
    4. Open %ProgramData%\VoltioVolt\logs\vltd.log.
    5. Uninstall with `.\dev-uninstall.ps1` (keep the data directory). Copy the output.
- Expected result: no vlt-setup step reports an error. Status "Running". Required privileges SeShutdownPrivilege and SeChangeNotifyPrivilege. Recovery: restart, restart, none. The ACL has SYSTEM, Administrators and NT SERVICE\vltd only. The log has `vault canary check passed state=available key_mode=dpapi`.
- Send: the output of steps 1, 2, 3 and 5, the log lines of step 4, the Windows version (`winver`).

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

- Purpose: check the Simulation of the default policy and of a changed policy (TASK 03-00-00-006) on the Owner computer.
- Preconditions: a computer with Go 1.27.3 and a checkout of `main`. No UPS and no administrator rights are necessary. These tests use only the simulator and dry-run or fake executors; they cannot turn off the computer.
- Steps:
    1. `go test -count=1 -v -run 'TestScenarios|TestActivationRules' ./internal/policy`
    2. `go test -count=1 -v -run TestPowerFailureIntegration ./cmd/vltd` (the integration test client: about 25 seconds).
    3. Later, with the GUI: policy.get, policy.simulate, policy.activate, then change countdown_seconds with policy.set and check that the policy is inactive.
- Expected result: all tests PASS. Default policy: power_fail_long gives a shutdown request at 370 s, power_fail_short gives a cancel, low_battery gives a refused cancel and a request at 90 s. After a change, the policy is inactive until a new Simulation passes.
- Send: the full output of steps 1 and 2, `go version` and the OS.

### T-09 Real shutdown with the Reference UPS

WARNING: This test turns off the computer. Save all work. Close all applications. Make sure that the UPS battery is fully charged. Set trigger.on_battery_seconds to 30. Disconnect the UPS input power. Connect the input power again after the computer is off.

- Purpose: check the full chain on real hardware: UPS, Device Manager, Event Engine, Policy Engine, OS shutdown executor (TASK 03-00-00-006 and 007).
- Preconditions: T-01 to T-03 and T-07 are complete. The computer is powered by the Reference UPS, and the UPS is connected to the computer with USB or serial. vltd runs (T-02 or T-03). An IPC client to set the policy (the later GUI, or a client that the Instruction Author gives). No other person uses the computer.
- Steps:
    1. With the IPC client: auth.login, then policy.set with `trigger.on_battery_seconds` = 30 (keep the other defaults: countdown 60 s, action shutdown).
    2. policy.simulate. The result must be "passed". Then policy.activate.
    3. device.status: the state must be "connected".
    4. status.subscribe. Write down the time.
    5. Disconnect the UPS input power. Write down the time.
    6. Watch the push messages: ON_BATTERY, after 30 seconds the countdown start (trigger on_battery_seconds), then 60 seconds countdown.
    7. The computer turns off. Write down the time.
    8. Connect the UPS input power again. Start the computer.
    9. Linux: `journalctl -u vltd -b -1 --no-pager | tail -100`. Windows: vltd.log, and Event Viewer, System log, source User32, event ID 1074.
    10. Read the audit log with audit.list (policy_countdown_start, policy_shutdown_request, os_shutdown_request).
- Expected result: the computer turns off about 90 seconds after step 5, without a dialog. The audit log has the three entries. Linux: os_shutdown_request has the inhibitor list. Windows: event 1074 shows the reason code 0x6000c ("Power Failure: Environment"; the exact event text is UNVERIFIED).
- Abort: to stop the countdown, connect the input power again before the end (cancel_when_mains_returns is on), or use shutdown.cancel (not possible with LOW_BATTERY).
- Send: the times of steps 4, 5 and 7, the log lines, the audit entries (no Secret is in them), and the Windows event 1074 text.

## T. Türkçe

Her testte şunlar vardır: amaç, ön koşullar, adımlar, beklenen sonuç ve Instruction Author'a gönderilecekler. Testleri sırayla yapın. Her testin sonuçlarını test biter bitmez gönderin. Secret, PIN, DEK, QID değeri veya seri numarası göndermeyin. Göndermeden önce her dosyayı kontrol edin.

Tüm testler için genel ön koşullar:

- Üretim bilgisayarı değil, bir test bilgisayarı. Her testten önce tüm çalışmalarınızı kaydedin.
- Referans UPS ve USB veya seri kablosu.
- `main` dalındaki son başarılı `ci` çalışmasının CI artifact dosyaları; SHA-256 kontrolü docs/user/probe.md bölüm 1'e göre yapılmış olmalıdır.

### T-01 Referans UPS ile vlt-probe Capture

- Amaç: Referans UPS'in Dialect'ini ve gerçek yanıtlarını bulmak (VLT-IS-002 TASK 03-00-00-008).
- Ön koşullar: CI artifact dosyasından vlt-probe. UPS, USB ile bağlıdır. Mümkünse Linux kullanın, çünkü Windows'ta ham rapor tanımlayıcısı (raw report descriptor) yoktur. UPS bataryası tam doludur.
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
- Beklenen sonuç: "active (running)". Günlükte `vault canary check passed state=available key_mode=host+tpm2` (TPM2 ile) veya `state="reduced protection" key_mode=host` (TPM2 olmadan), `audit chain verification ok=true`, `ipc server started` satırları vardır. UPS bağlı değilse: aygıt durumu "no device" olur ve vltd'yi durduran bir hata olmaz.
- Gönderin: 2. ve 4. adımların çıktısı, 3. ve 5. adımların günlük satırları (gerekirse bilgisayar adlarını silin), systemd sürümü (`systemctl --version`), TPM2 var mı yok mu.

### T-03 Windows'ta vlt-setup ile vltd geliştirme kurulumu

- Amaç: vlt-setup aracını (TASK 03-00-00-002) gerçek bir Windows bilgisayarında kontrol etmek.
- Ön koşullar: Windows 10 veya 11 ya da Windows Server. Bir Administrator hesabı. SHA-256 kontrolü yapılmış `vltd-vlt-setup-windows-amd64` artifact dosyası.
- Adımlar:
    1. docs/user/dev-service.md E.3 (Türkçe: T.3) ile kurun. vlt-setup çıktısının tamamını kopyalayın.
    2. `Get-Service vltd`, `sc.exe qprivs vltd` ve `sc.exe qfailure vltd`.
    3. `icacls "$env:ProgramData\VoltioVolt"`.
    4. %ProgramData%\VoltioVolt\logs\vltd.log dosyasını açın.
    5. `.\dev-uninstall.ps1` ile kaldırın (veri klasörünü koruyun). Çıktıyı kopyalayın.
- Beklenen sonuç: vlt-setup adımlarının hiçbiri hata bildirmez. Durum "Running". Gerekli ayrıcalıklar SeShutdownPrivilege ve SeChangeNotifyPrivilege. Kurtarma: restart, restart, none. ACL'de yalnızca SYSTEM, Administrators ve NT SERVICE\vltd vardır. Günlükte `vault canary check passed state=available key_mode=dpapi` vardır.
- Gönderin: 1, 2, 3 ve 5. adımların çıktısı, 4. adımın günlük satırları, Windows sürümü (`winver`).

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

- Amaç: varsayılan politikanın ve değiştirilmiş bir politikanın Simülasyonunu (TASK 03-00-00-006) Owner bilgisayarında kontrol etmek.
- Ön koşullar: Go 1.27.3 olan bir bilgisayar ve `main` dalının bir kopyası. UPS ve yönetici hakları gerekmez. Bu testler yalnızca simülatörü ve dry-run veya sahte (fake) yürütücüleri kullanır; bilgisayarı kapatamazlar.
- Adımlar:
    1. `go test -count=1 -v -run 'TestScenarios|TestActivationRules' ./internal/policy`
    2. `go test -count=1 -v -run TestPowerFailureIntegration ./cmd/vltd` (entegrasyon testi istemcisi: yaklaşık 25 saniye).
    3. Daha sonra, GUI ile: policy.get, policy.simulate, policy.activate; sonra policy.set ile countdown_seconds değerini değiştirin ve politikanın etkin olmadığını kontrol edin.
- Beklenen sonuç: tüm testler PASS. Varsayılan politika: power_fail_long 370. saniyede bir kapatma isteği verir, power_fail_short bir iptal verir, low_battery reddedilen bir iptal ve 90. saniyede bir istek verir. Bir değişiklikten sonra, yeni bir Simülasyon başarılı olana kadar politika etkin değildir.
- Gönderin: 1. ve 2. adımların tam çıktısı, `go version` ve işletim sistemi.

### T-09 Referans UPS ile gerçek kapatma

UYARI: Bu test bilgisayarı kapatır. Tüm çalışmalarınızı kaydedin. Tüm uygulamaları kapatın. UPS bataryasının tam dolu olduğundan emin olun. trigger.on_battery_seconds değerini 30 yapın. UPS giriş gücünü kesin. Bilgisayar kapandıktan sonra giriş gücünü yeniden bağlayın.

(English, verbatim: "WARNING: This test turns off the computer. Save all work. Close all applications. Make sure that the UPS battery is fully charged. Set trigger.on_battery_seconds to 30. Disconnect the UPS input power. Connect the input power again after the computer is off.")

- Amaç: tüm zinciri gerçek donanımda kontrol etmek: UPS, Device Manager, Event Engine, Policy Engine, işletim sistemi kapatma yürütücüsü (TASK 03-00-00-006 ve 007).
- Ön koşullar: T-01 ile T-03 ve T-07 tamamlanmıştır. Bilgisayar Referans UPS'ten beslenir ve UPS bilgisayara USB veya seri kablo ile bağlıdır. vltd çalışır (T-02 veya T-03). Politikayı ayarlamak için bir IPC istemcisi (daha sonraki GUI veya Instruction Author'un verdiği bir istemci). Bilgisayarı başka kimse kullanmaz.
- Adımlar:
    1. IPC istemcisi ile: auth.login, sonra policy.set ile `trigger.on_battery_seconds` = 30 (diğer varsayılanları koruyun: geri sayım 60 s, eylem shutdown).
    2. policy.simulate. Sonuç "passed" olmalıdır. Sonra policy.activate.
    3. device.status: durum "connected" olmalıdır.
    4. status.subscribe. Zamanı not edin.
    5. UPS giriş gücünü kesin. Zamanı not edin.
    6. Push mesajlarını izleyin: ON_BATTERY, 30 saniye sonra geri sayımın başlaması (tetikleyici on_battery_seconds), sonra 60 saniyelik geri sayım.
    7. Bilgisayar kapanır. Zamanı not edin.
    8. UPS giriş gücünü yeniden bağlayın. Bilgisayarı başlatın.
    9. Linux: `journalctl -u vltd -b -1 --no-pager | tail -100`. Windows: vltd.log ve Olay Görüntüleyicisi, Sistem günlüğü, kaynak User32, olay kimliği 1074.
    10. audit.list ile denetim günlüğünü okuyun (policy_countdown_start, policy_shutdown_request, os_shutdown_request).
- Beklenen sonuç: bilgisayar 5. adımdan yaklaşık 90 saniye sonra, bir iletişim kutusu olmadan kapanır. Denetim günlüğünde üç kayıt vardır. Linux: os_shutdown_request kaydında engelleyici (inhibitor) listesi vardır. Windows: 1074 olayı 0x6000c neden kodunu gösterir ("Power Failure: Environment"; olay metninin tam hali UNVERIFIED).
- Durdurma: geri sayımı durdurmak için, süre bitmeden giriş gücünü yeniden bağlayın (cancel_when_mains_returns açıktır) veya shutdown.cancel kullanın (LOW_BATTERY etkinken mümkün değildir).
- Gönderin: 4, 5 ve 7. adımların zamanları, günlük satırları, denetim kayıtları (içlerinde Secret yoktur) ve Windows 1074 olay metni.
