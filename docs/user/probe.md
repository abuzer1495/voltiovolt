# vlt-probe user guide / vlt-probe kullanım kılavuzu

- [English](#english)
- [Türkçe](#türkçe)

---

## English

vlt-probe is a READ-only capture tool. It sends only the commands of the Probe List and the Watch List to the UPS. These lists are constants in the code. vlt-probe has no option to send other commands.

Probe List (fixed order): `Q1, F, I, QS, QGS, QMOD, QBV, QWS, QFLAG, QRI, QID, QVFW, QMD` (VLT-IS-006)

Watch List: `Q1, QS, QGS, QMOD, QBV, QWS`

### 1. Download the artifact and check the SHA-256

1. Sign in to GitHub. GitHub shows artifacts only to signed-in users.
2. Open the **Actions** tab of the repository. Select the latest successful **ci** run on `main`.
3. In the **Artifacts** section, download `vlt-probe-windows-amd64` or `vlt-probe-linux-amd64`. The download is a ZIP file. Extract it.
4. The ZIP file contains the program and a `.sha256` file. Check the SHA-256:
   - Linux:
     ```
     sha256sum -c vlt-probe-linux-amd64.sha256
     chmod +x vlt-probe-linux-amd64
     ```
     The result must be `vlt-probe-linux-amd64: OK`.
   - Windows (PowerShell):
     ```
     Get-FileHash .\vlt-probe-windows-amd64.exe -Algorithm SHA256
     Get-Content .\vlt-probe-windows-amd64.exe.sha256
     ```
     The two hash values must be the same (upper case and lower case are the same).
5. If the values are not the same, do not run the program. Tell the Owner.

NOTE: An artifact is not a release. GitHub deletes it after 14 days. The program is not signed.

### 2. Run scan, probe and watch

Close all other UPS software (for example UPSilon 2000) before you start. Only one program can use the port.

| Mode | What it does | Example |
|------|--------------|---------|
| `scan` | Lists the HID devices with VID 0665 and PID 5161, and the serial ports. Sends no command. | `vlt-probe scan` |
| `probe` | Sends each Probe List command one time, in order. | `vlt-probe probe --transport hid` |
| `watch` | Sends the Watch List again and again for 1 to 30 minutes. | `vlt-probe watch --transport hid --minutes 5` |

Flags:

| Flag | Values | Note |
|------|--------|------|
| `--transport` | `hid` (default) or `serial` | |
| `--port` | serial port name, for example `COM3` or `/dev/ttyUSB0` | serial only |
| `--minutes` | 1 to 30 (default 1) | watch only |
| `--dtr` | `unchanged` (default), `on` or `off`. With `unchanged`, vlt-probe does not set the line (on Windows the serial library sets it on at open; refer to docs/protocol.md P-121). | serial only |
| `--rts` | `unchanged` (default), `on` or `off`. Same rule as `--dtr`. | serial only |

Serial example: `vlt-probe probe --transport serial --port COM3`

Timing: vlt-probe waits 500 ms between commands. The timeout of each command is 3 seconds. A Watch List cycle starts every 2 seconds. If a cycle takes more time (6 commands with 500 ms between them), the next cycle starts immediately after it. If a command returns "not supported" two times, watch mode does not send it again.

To stop watch mode before the end, push Ctrl+C. vlt-probe still writes the Capture file.

At the end, vlt-probe shows a short summary and the name of the Capture file: `vlt-capture-<UTC date and time>.vltcap.json`. The file is in the current directory.

### 3. Linux: hidraw needs root

The HID device file (`/dev/hidrawN`) needs root access. Use sudo for vlt-probe only:

```
sudo ./vlt-probe-linux-amd64 probe --transport hid
```

Do not run other programs as root for this test. After the test, you can change the owner of the Capture file:

```
sudo chown "$USER" vlt-capture-*.vltcap.json
```

### 4. Mains test in watch mode

> **WARNING: Make sure that the battery is fully charged. Disconnect the UPS input power for 60 seconds or less. The protected equipment runs on battery during this time. Then connect the input power again.**

1. Start watch mode: `vlt-probe watch --transport hid --minutes 3`
2. Wait 30 seconds.
3. Disconnect the UPS input power (the wall plug of the UPS). Write down the time.
4. Wait 60 seconds or less.
5. Connect the input power again. Write down the time.
6. Let watch mode run to the end.
7. Send the Capture file and the two times to the Owner.

### 5. Send the Capture file to the Owner channel

1. Open the Capture file in a text editor. Check the file before you send it.
2. Make sure that it contains no personal data. vlt-probe replaces the QID data and the USB serial number with `REDACTED` and the length. vlt-probe does not record the host name or the user name. If you see other personal data, remove it or tell the Owner.
3. Send the file to the Owner channel. The Owner tells you which channel to use.
4. Do not publish the Capture file in a public issue or pull request.

---

## Türkçe

vlt-probe yalnızca okuma (READ) yapan bir kayıt aracıdır. UPS'e yalnızca Probe List ve Watch List komutlarını gönderir. Bu listeler kodun içinde sabittir. vlt-probe başka bir komut gönderme seçeneğine sahip değildir.

Probe List (sabit sıra): `Q1, F, I, QS, QGS, QMOD, QBV, QWS, QFLAG, QRI, QID, QVFW, QMD` (VLT-IS-006)

Watch List: `Q1, QS, QGS, QMOD, QBV, QWS`

### 1. Artifact dosyasını indirin ve SHA-256 değerini kontrol edin

1. GitHub'a giriş yapın. GitHub, artifact dosyalarını yalnızca giriş yapmış kullanıcılara gösterir.
2. Deponun **Actions** sekmesini açın. `main` dalındaki son başarılı **ci** çalışmasını seçin.
3. **Artifacts** bölümünden `vlt-probe-windows-amd64` veya `vlt-probe-linux-amd64` dosyasını indirin. İndirilen dosya bir ZIP dosyasıdır. Dosyayı açın.
4. ZIP dosyasında program ve bir `.sha256` dosyası vardır. SHA-256 değerini kontrol edin:
   - Linux:
     ```
     sha256sum -c vlt-probe-linux-amd64.sha256
     chmod +x vlt-probe-linux-amd64
     ```
     Sonuç `vlt-probe-linux-amd64: OK` olmalıdır.
   - Windows (PowerShell):
     ```
     Get-FileHash .\vlt-probe-windows-amd64.exe -Algorithm SHA256
     Get-Content .\vlt-probe-windows-amd64.exe.sha256
     ```
     İki değer aynı olmalıdır (büyük harf ve küçük harf farkı önemli değildir).
5. Değerler aynı değilse programı çalıştırmayın. Owner'a bildirin.

NOT: Artifact bir sürüm (release) değildir. GitHub bu dosyayı 14 gün sonra siler. Program imzalı değildir.

### 2. scan, probe ve watch modlarını çalıştırın

Başlamadan önce diğer tüm UPS yazılımlarını (örneğin UPSilon 2000) kapatın. Bağlantı noktasını aynı anda yalnızca bir program kullanabilir.

| Mod | Ne yapar | Örnek |
|-----|----------|-------|
| `scan` | VID 0665 ve PID 5161 olan HID cihazlarını ve seri bağlantı noktalarını listeler. Komut göndermez. | `vlt-probe scan` |
| `probe` | Probe List komutlarının her birini sırayla bir kez gönderir. | `vlt-probe probe --transport hid` |
| `watch` | Watch List komutlarını 1 ile 30 dakika boyunca tekrar tekrar gönderir. | `vlt-probe watch --transport hid --minutes 5` |

Seçenekler:

| Seçenek | Değerler | Not |
|---------|----------|-----|
| `--transport` | `hid` (varsayılan) veya `serial` | |
| `--port` | seri bağlantı noktası adı, örneğin `COM3` veya `/dev/ttyUSB0` | yalnızca serial |
| `--minutes` | 1 ile 30 arası (varsayılan 1) | yalnızca watch |
| `--dtr` | `unchanged` (varsayılan), `on` veya `off`. `unchanged` ile vlt-probe hattı ayarlamaz (Windows'ta seri kitaplık açılışta hattı açık yapar; docs/protocol.md P-121). | yalnızca serial |
| `--rts` | `unchanged` (varsayılan), `on` veya `off`. `--dtr` ile aynı kural. | yalnızca serial |

Seri bağlantı örneği: `vlt-probe probe --transport serial --port COM3`

Zamanlama: vlt-probe komutlar arasında 500 ms bekler. Her komutun zaman aşımı 3 saniyedir. Her Watch List döngüsü 2 saniyede bir başlar. Bir döngü daha uzun sürerse (aralarında 500 ms olan 6 komut), sonraki döngü hemen ardından başlar. Bir komut iki kez "not supported" (desteklenmiyor) sonucu verirse, watch modu bu komutu bir daha göndermez.

watch modunu süre bitmeden durdurmak için Ctrl+C tuşlarına basın. vlt-probe yine de Capture dosyasını yazar.

Sonunda vlt-probe kısa bir özet (İngilizce) ve Capture dosyasının adını gösterir: `vlt-capture-<UTC tarih ve saat>.vltcap.json`. Dosya, programı çalıştırdığınız klasördedir.

### 3. Linux: hidraw için root yetkisi gerekir

HID cihaz dosyası (`/dev/hidrawN`) root yetkisi ister. sudo'yu yalnızca vlt-probe için kullanın:

```
sudo ./vlt-probe-linux-amd64 probe --transport hid
```

Bu test için başka programları root olarak çalıştırmayın. Testten sonra Capture dosyasının sahibini değiştirebilirsiniz:

```
sudo chown "$USER" vlt-capture-*.vltcap.json
```

### 4. watch modunda şebeke testi

> **UYARI: Akünün tam dolu olduğundan emin olun. UPS giriş gücünü en fazla 60 saniye süreyle kesin. Bu süre boyunca korunan cihazlar aküyle çalışır. Sonra giriş gücünü tekrar bağlayın.**
>
> (English original: "WARNING: Make sure that the battery is fully charged. Disconnect the UPS input power for 60 seconds or less. The protected equipment runs on battery during this time. Then connect the input power again.")

1. watch modunu başlatın: `vlt-probe watch --transport hid --minutes 3`
2. 30 saniye bekleyin.
3. UPS giriş gücünü kesin (UPS'in duvar fişini çekin). Saati not edin.
4. En fazla 60 saniye bekleyin.
5. Giriş gücünü tekrar bağlayın. Saati not edin.
6. watch modunun sonuna kadar çalışmasını bekleyin.
7. Capture dosyasını ve not ettiğiniz iki saati Owner'a gönderin.

### 5. Capture dosyasını Owner kanalına gönderin

1. Capture dosyasını bir metin düzenleyiciyle açın. Göndermeden önce dosyayı kontrol edin.
2. Dosyada kişisel veri olmadığından emin olun. vlt-probe, QID verisini ve USB seri numarasını `REDACTED` ve uzunluk bilgisiyle değiştirir. vlt-probe bilgisayar adını ve kullanıcı adını kaydetmez. Başka bir kişisel veri görürseniz silin veya Owner'a bildirin.
3. Dosyayı Owner kanalına gönderin. Hangi kanalın kullanılacağını Owner size söyler.
4. Capture dosyasını herkese açık bir issue veya pull request içinde yayınlamayın.
