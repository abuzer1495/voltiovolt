# UPS simulator / UPS simülatörü (VLT-IS-010)

- [English](#english)
- [Türkçe](#türkçe)

---

## English

vlt-simulator is a console UPS simulator. It runs in a classic cmd window (Windows) or a terminal (Linux). It is in the test package: `vlt-simulator.exe` on Windows, `./vlt-simulator` on Linux. The install script copies it next to VoltioVolt. There is no Start menu shortcut for the simulator.

The simulator is not a real UPS. It talks to VoltioVolt on a local stream (a Windows named pipe or a Linux Unix socket). It does not use TCP and it does not use a COM port. Only one instance can run.

### 1. Start

1. Start the simulator first:
   - Windows: double-click `vlt-simulator.exe` in the package folder (or after install: `%LOCALAPPDATA%\Programs\VoltioVolt\vlt-simulator.exe`), or run it in a cmd window.
   - Linux: in a terminal, run `./vlt-simulator` in the extracted folder (or after install: `~/.local/opt/voltiovolt/vlt-simulator`).
2. If a second instance starts, it exits with the message "The simulator already runs" (Turkish: "Simülatör zaten çalışıyor").
3. In VoltioVolt, open Device and select "Test connection (simulator)". A blue banner "TEST CONNECTION" is on all screens. VoltioVolt connects automatically. If the simulator is not running, VoltioVolt shows: "Start vlt-simulator.exe, then wait. VoltioVolt connects automatically." and tries again every 5 seconds.
4. Change values in the simulator with the keys below. Watch VoltioVolt: the values change at once.

The language is `--lang tr` or `--lang en`. The default is the OS language if it is Turkish, else English.

`--headless` has no screen. It is for CI. With `--headless`, `--scenario N` runs one scenario (1 to 9). `--scenario-delay` waits first, so VoltioVolt can connect and record the normal values. `--dialect` is `megatec` (default), `voltronic-qs` or `voltronic`.

### 2. The screen

The screen has four parts:

- A title: "VoltioVolt UPS Simulator". The connection state is "waiting" or "connected". The endpoint is a local named pipe or a Unix socket. The last scenario, if any, is shown.
- A menu of 18 items (left). The selected item is highlighted. The current value of the selected item is next to the name.
- A status panel (right): input voltage, input fault voltage, output voltage, frequency, load, battery charge, battery voltage, temperature, runtime, status bits, output on/off, and timers (test, shutdown, restore).
- A key line at the bottom. Key L opens a full log of the last commands and responses (L closes it; Up and Down scroll).

### 3. Keys

| Key | Action |
|-----|--------|
| Up, Down | Select a menu item |
| Left, Right | Change the value by one step |
| Page Up, Page Down | Change the value by ten steps |
| Enter or Space | Switch an on/off item, or run an action |
| 1 to 9 | Run a scenario |
| R | Reset to the default values |
| L | Show or hide the full log |
| Q or Esc | Quit (asks for confirmation: "Quit? [Y/N]") |

### 4. Menu items

In order:

1. Mains power (on/off)
2. Input voltage
3. Input frequency
4. Output voltage (on mains)
5. Load
6. Battery charge
7. Battery speed
8. Temperature
9. UPS type (on-line or standby)
10. Battery low (b6) (auto or forced on)
11. UPS fault (b4) (on/off)
12. Bypass or AVR (b5) (on/off)
13. Beeper (b0) (on/off)
14. Dialect (megatec, voltronic-qs, voltronic)
15. Reply to other commands (echo, NAK, none)
16. Response delay
17. Fault injection (none, no response, garbage, split, wrong length, old bytes)
18. Rated values (F)

### 5. Scenarios

Press a number key:

| Key | Scenario |
|-----|----------|
| 1 | Power failure now (mains off; stays off until you turn mains on or run scenario 9 or R) |
| 2 | Power failure for 2 minutes, then mains on |
| 3 | Low input voltage: 180 V |
| 4 | High input voltage: 260 V |
| 5 | Overload: 110 % |
| 6 | Battery low: charge 15 % and mains off |
| 7 | Communication loss for 30 seconds (no response), then normal |
| 8 | High temperature: 55 C |
| 9 | Back to normal (the same as R: default values) |

### 6. Connection to VoltioVolt

VoltioVolt uses the real Driver (the same Exchange, Probe and Poller as a real UPS). The protocol is the same bytes as on a serial line (ASCII command and CR). Measurements and Events have the source "test". History and Events show the current source by default; the filter "Show other sources" shows the other sources (real, demo, test). Notifications have the prefix "[TEST]".

The OS shutdown is a dry run by default. At the end of a countdown the message is: "Test: the computer would shut down now." The Setting "Permit a real computer shutdown in a test" (`test.real_shutdown`) is off by default. The Device screen shows this WARNING if you turn it on: "With this setting on, a test can turn off this computer." Leave it off unless you have a reason to do a real shutdown.

CONTROL commands (tests, beeper, shutdown of the UPS output) go to the simulator only, never to a real UPS and never to a real port.

To leave the test connection, select a different transport. The confirmation is: "Leave the test connection and connect to a real UPS? Events and measurements of the test stay, marked as test."

---

### 7. Owner tests

- [T-17 Simulator test](owner-tests.md#t-17-simulator-test-vlt-is-010)
- [T-18 Simple mode and Expert mode](owner-tests.md#t-18-simple-mode-and-expert-mode-vlt-is-011)
- [T-20 Live History charts with the simulator](owner-tests.md#t-20-live-history-charts-with-the-simulator-vlt-is-011)

## Türkçe

vlt-simulator bir konsol UPS simülatörüdür. Klasik bir cmd penceresinde (Windows) veya bir uçbirimde (Linux) çalışır. Test paketindedir: Windows'ta `vlt-simulator.exe`, Linux'ta `./vlt-simulator`. Kurulum betiği onu VoltioVolt'un yanına kopyalar. Simülatör için bir Başlat menüsü kısayolu yoktur.

Simülatör gerçek bir UPS değildir. VoltioVolt ile yerel bir akış üzerinde konuşur (Windows adlandırılmış boru veya Linux Unix soketi). TCP kullanmaz ve bir COM portu kullanmaz. Aynı anda yalnızca bir örnek çalışabilir.

### 1. Başlatma

1. Önce simülatörü başlatın:
   - Windows: paket klasöründe `vlt-simulator.exe` dosyasına çift tıklayın (veya kurulumdan sonra: `%LOCALAPPDATA%\Programs\VoltioVolt\vlt-simulator.exe`), veya bir cmd penceresinde çalıştırın.
   - Linux: bir uçbirimde, çıkarılan klasörde `./vlt-simulator` çalıştırın (veya kurulumdan sonra: `~/.local/opt/voltiovolt/vlt-simulator`).
2. İkinci bir örnek başlarsa, "Simülatör zaten çalışıyor" iletisi ile çıkar (İngilizce: "The simulator already runs").
3. VoltioVolt'ta Aygıt ekranını açın ve "Test bağlantısı (simülatör)" seçin. Tüm ekranlarda mavi bir "TEST BAĞLANTISI" bandı görünür. VoltioVolt otomatik bağlanır. Simülatör çalışmıyorsa VoltioVolt şunu gösterir: "vlt-simulator.exe uygulamasını başlatın, sonra bekleyin. VoltioVolt otomatik bağlanır." ve her 5 saniyede yeniden dener.
4. Simülatörde değerleri aşağıdaki tuşlarla değiştirin. VoltioVolt'u izleyin: değerler hemen değişir.

Dil `--lang tr` veya `--lang en` ile seçilir. Varsayılan: işletim sistemi dili Türkçe ise Türkçe, değilse İngilizce.

`--headless` ekran açmaz. CI içindir. `--headless` ile `--scenario N` bir senaryoyu çalıştırır (1 - 9). `--scenario-delay` önce bekler, böylece VoltioVolt bağlanıp normal değerleri kaydedebilir. `--dialect` şunlardan biridir: `megatec` (varsayılan), `voltronic-qs` veya `voltronic`.

### 2. Ekran

Ekranın dört bölümü vardır:

- Başlık: "VoltioVolt UPS Simülatörü". Bağlantı durumu "bekliyor" veya "bağlı"dır. Uç nokta yerel bir adlandırılmış boru veya bir Unix soketidir. Varsa son senaryo gösterilir.
- 18 öğelik bir menü (sol). Seçili öğe vurgulanır. Seçili öğenin güncel değeri adın yanındadır.
- Durum paneli (sağ): giriş gerilimi, giriş arıza gerilimi, çıkış gerilimi, frekans, yük, akü şarjı, akü gerilimi, sıcaklık, süre, durum bitleri, çıkış açık/kapalı ve zamanlayıcılar (test, kapatma, geri açma).
- Altta tuş satırı. L tuşu son komutların ve yanıtların tam günlüğünü açar (L kapatır; Yukarı ve Aşağı kaydırır).

### 3. Tuşlar

| Tuş | İşlem |
|-----|--------|
| Yukarı, Aşağı | Menü öğesini seç |
| Sol, Sağ | Değeri bir adım değiştir |
| Page Up, Page Down | Değeri on adım değiştir |
| Enter veya Boşluk | Aç/kapa öğesini değiştir, veya bir işlemi çalıştır |
| 1 - 9 | Bir senaryoyu çalıştır |
| R | Varsayılan değerlere sıfırla |
| L | Tam günlüğü göster veya gizle |
| Q veya Esc | Çık (onay ister: "Çıkılsın mı? [E/H]") |

### 4. Menü öğeleri

Sırayla:

1. Şebeke gücü (açık/kapalı)
2. Giriş gerilimi
3. Giriş frekansı
4. Çıkış gerilimi (şebekede)
5. Yük
6. Akü şarjı
7. Akü hızı
8. Sıcaklık
9. UPS tipi (on-line veya yedekleme)
10. Akü düşük (b6) (otomatik veya zorunlu açık)
11. UPS arızası (b4) (açık/kapalı)
12. Bypass veya AVR (b5) (açık/kapalı)
13. Sesli uyarı (b0) (açık/kapalı)
14. Diyalekt (megatec, voltronic-qs, voltronic)
15. Diğer komutlara yanıt (yankı, NAK, yok)
16. Yanıt gecikmesi
17. Arıza enjeksiyonu (yok, yanıt yok, çöp, bölünmüş, yanlış uzunluk, eski baytlar)
18. Anma değerleri (F)

### 5. Senaryolar

Bir sayı tuşuna basın:

| Tuş | Senaryo |
|-----|----------|
| 1 | Şimdi elektrik kesintisi (şebeke kapalı; şebekeyi açana, 9. senaryoyu veya R'yi çalıştırana kadar kapalı kalır) |
| 2 | 2 dakikalık kesinti, sonra şebeke açık |
| 3 | Düşük giriş gerilimi: 180 V |
| 4 | Yüksek giriş gerilimi: 260 V |
| 5 | Aşırı yük: % 110 |
| 6 | Akü düşük: şarj % 15 ve şebeke kapalı |
| 7 | 30 saniyelik haberleşme kaybı (yanıt yok), sonra normal |
| 8 | Yüksek sıcaklık: 55 C |
| 9 | Normal değerlere dönüş (R ile aynı: varsayılan değerler) |

### 6. VoltioVolt bağlantısı

VoltioVolt gerçek Sürücüyü kullanır (gerçek bir UPS ile aynı Exchange, Probe ve Poller). Protokol, seri hat ile aynı baytlardır (ASCII komut ve CR). Ölçümler ve Olaylar "test" kaynağına sahiptir. Geçmiş ve Olaylar varsayılan olarak güncel kaynağı gösterir; "Diğer kaynakları göster" süzgeci diğer kaynakları (gerçek, demo, test) gösterir. Bildirimlerin ön eki "[TEST]" olur.

İşletim sistemi kapanması varsayılan olarak kuru çalıştırmadır. Geri sayımın sonundaki ileti şudur: "Test: bilgisayar şimdi kapanırdı." "Bir testte gerçek bilgisayar kapanmasına izin ver" ayarı (`test.real_shutdown`) varsayılan olarak kapalıdır. Açarsanız Aygıt ekranı şu UYARIYI gösterir: "Bu ayar açıkken bir test bu bilgisayarı kapatabilir." Gerçek bir kapanma için bir nedeniniz yoksa bu ayarı kapalı bırakın.

CONTROL komutları (testler, sesli uyarı, UPS çıkışının kapanması) yalnızca simülatöre gider; gerçek bir UPS'e ve gerçek bir porta gitmez.

Test bağlantısından çıkmak için farklı bir taşıma seçin. Onay metni şudur: "Test bağlantısından çıkılsın ve gerçek bir UPS bağlantısı kullanılsın mı? Test olayları ve ölçümleri geçmişte kalır ve test olarak işaretlenir."

### 7. Sahip testleri

- [T-17 Simülatör testi](owner-tests.md#t-17-simülatör-testi-vlt-is-010)
- [T-18 Basit kip ve Uzman kip](owner-tests.md#t-18-basit-kip-ve-uzman-kip-vlt-is-011)
- [T-20 Simülatörle canlı Geçmiş grafikleri](owner-tests.md#t-20-simülatörle-canlı-geçmiş-grafikleri-vlt-is-011)
