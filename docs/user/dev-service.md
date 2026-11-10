# vltd development service: install and test (VLT-IS-003 TASK 03-00-00-008, VLT-IS-004 TASK 03-00-00-001)

English first. Turkish: section T below.

WARNING: This is a development install, not a package. vltd does not talk to a UPS yet. Use a test computer.

## E. English

### E.1 Build

On a development computer with Go 1.27.3:

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

vlt-dev-init.sh makes the users and groups, the encrypted DEK /etc/credstore.encrypted/vlt-dek and /etc/vltd/key-mode. It also installs the polkit rule /etc/polkit-1/rules.d/50-vltd.rules (owner root, mode 0644): only the user vltd can power off or hibernate the computer. Do not enable vltd-fallback.service. To use the IPC as a normal user, add the user to the group vlt: `usermod -aG vlt <user>` (then log in again).

### E.3 Install on Windows

Open PowerShell as Administrator in the folder with vltd.exe and the script:

    Set-ExecutionPolicy -Scope Process Bypass
    .\packaging\windows\dev-install.ps1 -Binary .\vltd.exe

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
    - Windows: %ProgramData%\VoltioVolt (vltd.db and vault.dek). Then run the icacls command of dev-install.ps1 again.
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

- Windows (as Administrator): `.\packaging\windows\dev-uninstall.ps1`. The script asks before it deletes the data directory.

## T. Türkçe

### T.1 Derleme

Go 1.27.3 olan bir geliştirme bilgisayarında:

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

vlt-dev-init.sh kullanıcıları ve grupları, şifreli DEK dosyasını (/etc/credstore.encrypted/vlt-dek) ve /etc/vltd/key-mode dosyasını oluşturur. Ayrıca polkit kuralını /etc/polkit-1/rules.d/50-vltd.rules olarak kurar (sahip root, izin 0644): bilgisayarı yalnızca vltd kullanıcısı kapatabilir veya hazırda bekletebilir. vltd-fallback.service birimini etkinleştirmeyin (enable). IPC'yi normal bir kullanıcı ile kullanmak için kullanıcıyı vlt grubuna ekleyin: `usermod -aG vlt <kullanıcı>` (sonra yeniden oturum açın).

### T.3 Windows kurulumu

vltd.exe ve betiğin olduğu klasörde PowerShell'i Yönetici olarak açın:

    Set-ExecutionPolicy -Scope Process Bypass
    .\packaging\windows\dev-install.ps1 -Binary .\vltd.exe

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
    - Windows: %ProgramData%\VoltioVolt (vltd.db ve vault.dek). Sonra dev-install.ps1 içindeki icacls komutunu yeniden çalıştırın.
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

- Windows (Yönetici olarak): `.\packaging\windows\dev-uninstall.ps1`. Betik veri klasörünü silmeden önce sorar.
