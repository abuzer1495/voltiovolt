# vltd development service: install and test (VLT-IS-003 TASK 03-00-00-008, VLT-IS-004 TASK 03-00-00-001 and 03-00-00-002)

English first. Turkish: section T below.

WARNING: This is a development install, not a package. Use a test computer. Since VLT-IS-004, vltd finds and polls a connected UPS (READ commands only). The Shutdown Policy is inactive until a Simulation passes and an operator activates it (policy.activate). An active policy can turn off the computer.

## E. English

### E.1 Build

On a development computer with Go 1.27.4:

    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o vltd ./cmd/vltd
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o vltd.exe ./cmd/vltd

### E.2 Install on Linux (systemd 250 or later)

Run each command as root:

    install -m 0755 vltd /usr/local/bin/vltd
    sh packaging/linux/vlt-dev-init.sh
    install -m 0644 packaging/linux/vltd.service /etc/systemd/system/vltd.service
    install -m 0644 packaging/linux/vltd-fallback.service /etc/systemd/system/vltd-fallback.service
    systemctl daemon-reload
    systemctl start vltd

vlt-dev-init.sh makes the users and groups, the encrypted DEK /etc/credstore.encrypted/vlt-dek and /etc/vltd/key-mode. It also installs the polkit rule /etc/polkit-1/rules.d/50-vltd.rules (owner root, mode 0644): only the user vltd can power off or hibernate the computer. Do not enable vltd-fallback.service. To use the IPC (and the VLT GUI) as a normal user, the user must be in the group vlt. If you start vlt-dev-init.sh with sudo (`sudo sh packaging/linux/vlt-dev-init.sh`), the script adds the user who started sudo (SUDO_USER) to the group vlt (VLT-IS-005). Else: `usermod -aG vlt <user>`. Then log out and log in again.

### E.3 Install on Windows

Build vlt-setup too (`CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o vlt-setup.exe ./cmd/vlt-setup`), or download the CI artifact `vltd-vlt-setup-windows-amd64` and check the SHA-256 files. Put vltd.exe, vlt-setup.exe, dev-install.ps1 and dev-uninstall.ps1 in one folder. Open PowerShell as Administrator in that folder:

    Set-ExecutionPolicy -Scope Process Bypass
    .\dev-install.ps1

The script checks for Administrator rights and runs `vlt-setup.exe dev-install`. vlt-setup prints each step (VLT-IS-004 TASK 03-00-00-002):

1. Makes the local group "VLT Operators" (continues if it exists). 1a. Adds the user who started vlt-setup to the group (VLT-IS-005); sign out and sign in again to use the VLT GUI.
2. Copies vltd.exe to %ProgramFiles%\VoltioVolt and makes the service vltd with the virtual account NT SERVICE\vltd (start type manual).
3. Gives the user right SeShutdownPrivilege to NT SERVICE\vltd.
4. Sets the required privileges of the service: SeShutdownPrivilege and SeChangeNotifyPrivilege.
5. Makes %ProgramData%\VoltioVolt with the ACL of VLT-IS-003 TASK 03-00-00-003 and checks it.
6. Sets the recovery: restart after the first and the second failure, after 5 seconds.
7. Starts the service.

To give another user access to the IPC pipe, add the user to the group: `Add-LocalGroupMember -Group "VLT Operators" -Member <user>` (then log in again).

### E.4 Check the service status

- Linux: `systemctl status vltd`. The state must be "active (running)". Type=notify: "active" comes only after vltd sends READY=1.
- Windows: `Get-Service vltd`. The status must be "Running".

### E.5 Find the Vault state and the canary result in the log

- Linux: `journalctl -u vltd -b --no-pager`.
- Windows: `%ProgramData%\VoltioVolt\logs\vltd.log`.

Find these lines:

- `vault canary check passed state=available key_mode=host+tpm2` (or `state="reduced protection" key_mode=host` without a TPM2). At the first start: `vault made: canary stored`.
- `vault state state=...`.
- `audit chain verification ok=true rows=...`.
- `ipc server started`.

If the Vault cannot read the DEK, the line is `vault locked reason=...` or `vault not configured`. vltd continues; functions that need a Secret are off.

Linux note: if systemd cannot decrypt the credential, vltd.service stops with 243/CREDENTIALS and enters the "failed" state (no restart). After 3 other failed starts in 60 seconds, the unit also enters the "failed" state. Then systemd starts vltd-fallback.service (VLT_FALLBACK_MODE=1). The log shows `fallback mode: the primary unit failed` and `vault locked reason="primary unit failed"`. Check it with `systemctl status vltd-fallback`. Repair the credential, then run `systemctl reset-failed vltd` and `systemctl start vltd` (this stops the fallback unit).

### E.6 SEC-006 test: copy the data directory to a second computer

1. Stop vltd on computer A.
2. Install vltd on computer B (E.2 or E.3). Stop it.
3. Copy the data directory from A to B:
    - Linux: /var/lib/vltd (database vltd.db, vltd.db-wal, vltd.db-shm). Keep owner vltd and mode 0700/0600.
    - Windows: %ProgramData%\VoltioVolt (vltd.db and vault.dek). The copied files get the ACL of the folder. If vltd reports a permission error, run dev-uninstall.ps1 (keep the data directory) and dev-install.ps1 again: vlt-setup sets the ACL again.
4. Start vltd on B.
5. Look in the log of B. The Vault state MUST be "locked" (`vault locked reason="host binding failed"`). If the state is "available", stop and tell the Instruction Author: this is a security defect.

### E.7 Uninstall

- Linux (as root):

      systemctl stop vltd
      rm /etc/systemd/system/vltd.service /etc/systemd/system/vltd-fallback.service /etc/polkit-1/rules.d/50-vltd.rules /usr/local/bin/vltd
      systemctl daemon-reload
      # Optional, deletes all data and the DEK:
      rm -r /var/lib/vltd /etc/credstore.encrypted/vlt-dek /etc/vltd
      userdel vltd; groupdel vltd; groupdel vlt

- Windows (as Administrator): `.\dev-uninstall.ps1`. vlt-setup stops and removes the service, removes the user right SeShutdownPrivilege, removes vltd.exe, deletes the group "VLT Operators" only if it has no members, and asks before it deletes the data directory.

## T. Türkçe

UYARI: Bu bir geliştirme kurulumudur, paket değildir. Bir test bilgisayarı kullanın. VLT-IS-004 ile vltd bağlı bir UPS'i bulur ve sorgular (yalnızca READ komutları). Kapatma Politikası, bir Simülasyon başarılı olana ve bir operatör onu etkinleştirene (policy.activate) kadar etkin değildir. Etkin bir politika bilgisayarı kapatabilir.

### T.1 Derleme

Go 1.27.4 olan bir geliştirme bilgisayarında:

    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o vltd ./cmd/vltd
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o vltd.exe ./cmd/vltd

### T.2 Linux kurulumu (systemd 250 veya sonrası)

Her komutu root olarak çalıştırın:

    install -m 0755 vltd /usr/local/bin/vltd
    sh packaging/linux/vlt-dev-init.sh
    install -m 0644 packaging/linux/vltd.service /etc/systemd/system/vltd.service
    install -m 0644 packaging/linux/vltd-fallback.service /etc/systemd/system/vltd-fallback.service
    systemctl daemon-reload
    systemctl start vltd

vlt-dev-init.sh kullanıcıları ve grupları, şifreli DEK dosyasını (/etc/credstore.encrypted/vlt-dek) ve /etc/vltd/key-mode dosyasını oluşturur. Ayrıca polkit kuralını /etc/polkit-1/rules.d/50-vltd.rules olarak kurar (sahip root, izin 0644): bilgisayarı yalnızca vltd kullanıcısı kapatabilir veya hazırda bekletebilir. vltd-fallback.service birimini etkinleştirmeyin (enable). IPC'yi (ve VLT GUI'yi) normal bir kullanıcı ile kullanmak için kullanıcı vlt grubunda olmalıdır. vlt-dev-init.sh betiğini sudo ile başlatırsanız (`sudo sh packaging/linux/vlt-dev-init.sh`), betik sudo'yu başlatan kullanıcıyı (SUDO_USER) vlt grubuna ekler (VLT-IS-005). Aksi halde: `usermod -aG vlt <kullanıcı>`. Sonra oturumu kapatıp yeniden açın.

### T.3 Windows kurulumu

vlt-setup aracını da derleyin (`CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o vlt-setup.exe ./cmd/vlt-setup`) veya CI yapıtını (`vltd-vlt-setup-windows-amd64`) indirip SHA-256 dosyalarını kontrol edin. vltd.exe, vlt-setup.exe, dev-install.ps1 ve dev-uninstall.ps1 dosyalarını bir klasöre koyun. Bu klasörde PowerShell'i Yönetici olarak açın:

    Set-ExecutionPolicy -Scope Process Bypass
    .\dev-install.ps1

Betik Yönetici haklarını kontrol eder ve `vlt-setup.exe dev-install` komutunu çalıştırır. vlt-setup her adımı yazar (VLT-IS-004 TASK 03-00-00-002):

1. "VLT Operators" yerel grubunu oluşturur (varsa devam eder). 1a. vlt-setup aracını başlatan kullanıcıyı gruba ekler (VLT-IS-005); VLT GUI'yi kullanmak için oturumu kapatıp yeniden açın.
2. vltd.exe dosyasını %ProgramFiles%\VoltioVolt klasörüne kopyalar ve NT SERVICE\vltd sanal hesabıyla vltd hizmetini oluşturur (başlangıç türü el ile).
3. NT SERVICE\vltd hesabına SeShutdownPrivilege kullanıcı hakkını verir.
4. Hizmetin gerekli ayrıcalıklarını ayarlar: SeShutdownPrivilege ve SeChangeNotifyPrivilege.
5. %ProgramData%\VoltioVolt klasörünü VLT-IS-003 TASK 03-00-00-003 ACL'si ile oluşturur ve kontrol eder.
6. Kurtarmayı ayarlar: birinci ve ikinci hatadan sonra 5 saniye sonra yeniden başlat.
7. Hizmeti başlatır.

Başka bir kullanıcıya IPC kanalı erişimi vermek için kullanıcıyı gruba ekleyin: `Add-LocalGroupMember -Group "VLT Operators" -Member <kullanıcı>` (sonra yeniden oturum açın).

### T.4 Hizmet durumunu kontrol edin

- Linux: `systemctl status vltd`. Durum "active (running)" olmalıdır. Type=notify: "active" durumu yalnızca vltd READY=1 gönderdikten sonra gelir.
- Windows: `Get-Service vltd`. Durum "Running" olmalıdır.

### T.5 Günlükte Vault durumunu ve canary sonucunu bulun

- Linux: `journalctl -u vltd -b --no-pager`.
- Windows: `%ProgramData%\VoltioVolt\logs\vltd.log`.

Şu satırları bulun:

- `vault canary check passed state=available key_mode=host+tpm2` (TPM2 yoksa `state="reduced protection" key_mode=host`). İlk başlatmada: `vault made: canary stored`.
- `vault state state=...`.
- `audit chain verification ok=true rows=...`.
- `ipc server started`.

Vault DEK'i okuyamazsa satır `vault locked reason=...` veya `vault not configured` olur. vltd çalışmaya devam eder; Secret gerektiren işlevler kapalıdır.

Linux notu: systemd kimlik bilgisini (credential) çözemezse vltd.service 243/CREDENTIALS ile durur ve "failed" durumuna geçer (yeniden başlatma yok). 60 saniyede 3 başka başarısız başlatmadan sonra da birim "failed" durumuna geçer. Sonra systemd vltd-fallback.service birimini başlatır (VLT_FALLBACK_MODE=1). Günlükte `fallback mode: the primary unit failed` ve `vault locked reason="primary unit failed"` satırları görünür. `systemctl status vltd-fallback` ile kontrol edin. Kimlik bilgisini onarın, sonra `systemctl reset-failed vltd` ve `systemctl start vltd` komutlarını çalıştırın (bu, yedek birimi durdurur).

### T.6 SEC-006 testi: veri klasörünü ikinci bir bilgisayara kopyalayın

1. A bilgisayarında vltd'yi durdurun.
2. B bilgisayarına vltd kurun (T.2 veya T.3). Durdurun.
3. Veri klasörünü A'dan B'ye kopyalayın:
    - Linux: /var/lib/vltd (vltd.db, vltd.db-wal, vltd.db-shm). Sahip vltd, mod 0700/0600 kalmalıdır.
    - Windows: %ProgramData%\VoltioVolt (vltd.db ve vault.dek). Kopyalanan dosyalar klasörün ACL'sini alır. vltd bir izin hatası bildirirse dev-uninstall.ps1 (veri klasörünü koruyun) ve sonra dev-install.ps1 çalıştırın: vlt-setup ACL'yi yeniden ayarlar.
4. B'de vltd'yi başlatın.
5. B'nin günlüğüne bakın. Vault durumu "locked" OLMALIDIR (`vault locked reason="host binding failed"`). Durum "available" ise durun ve Talimat Yazarına bildirin: bu bir güvenlik hatasıdır.

### T.7 Kaldırma

- Linux (root olarak):

      systemctl stop vltd
      rm /etc/systemd/system/vltd.service /etc/systemd/system/vltd-fallback.service /etc/polkit-1/rules.d/50-vltd.rules /usr/local/bin/vltd
      systemctl daemon-reload
      # İsteğe bağlı, tüm verileri ve DEK'i siler:
      rm -r /var/lib/vltd /etc/credstore.encrypted/vlt-dek /etc/vltd
      userdel vltd; groupdel vltd; groupdel vlt

- Windows (Yönetici olarak): `.\dev-uninstall.ps1`. vlt-setup hizmeti durdurur ve kaldırır, SeShutdownPrivilege kullanıcı hakkını kaldırır, vltd.exe dosyasını siler, "VLT Operators" grubunu yalnızca üyesi yoksa siler ve veri klasörünü silmeden önce sorar.
