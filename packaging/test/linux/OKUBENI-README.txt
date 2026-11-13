VoltioVolt - TEST BUILD / TEST SÜRÜMÜ
=====================================

TÜRKÇE
------

Bu bir test sürümüdür. Normal kullanım için değildir. Bir test bilgisayarı kullanın.

Yöntem 1: "tar xzf VoltioVolt-test-linux-amd64.tar.gz", sonra "cd VoltioVolt-test-linux-amd64" ve "./voltiovolt". Kurulum gerekmez.

Yöntem 2: "sh install.sh". sudo gerekmez. Dosyalar ~/.local/opt/voltiovolt klasörüne kopyalanır. Onay verirseniz ~/.local/share/applications/voltiovolt.desktop kısayolu yapılır.

1. İndirme: GitHub'da Releases sayfasını açın, "Test build" sürümünü seçin, VoltioVolt-test-linux-amd64.tar.gz ve SHA256SUMS.txt dosyalarını indirin.
2. Kontrol: "sha256sum --ignore-missing -c SHA256SUMS.txt" komutu "OK" yazmalıdır.
3. Çalışma zamanı paketleri (Debian ve Ubuntu): "sudo apt install libgtk-4-1 libwebkitgtk-6.0-4".
4. UPS portunu başka bir program (örneğin UPSilon 2000) kullanmamalıdır.
5. Kullanıcınız /dev/ttyUSB* veya /dev/ttyACM* portunu açamazsa, bu komutu bir kez çalıştırın, sonra oturumu kapatıp açın: sudo usermod -aG dialout <kullanıcı adı>. VoltioVolt bu komutu çalıştırmaz.
6. Testler: https://github.com/abuzer1495/voltiovolt/blob/main/docs/user/owner-tests.md
7. Kaldırma: "sh uninstall.sh". Veri klasörü silinmeden önce ayrı bir soru sorulur.

UPS simulator
-------------

1. ./vlt-simulator programını bir uçbirimde başlatın (çıkarılan klasörde, veya kurulumdan sonra ~/.local/opt/voltiovolt/vlt-simulator).
2. VoltioVolt'ta Aygıt (Device) ekranını açın ve "Test bağlantısı (simülatör)" seçin.
3. Simülatörde değerleri ok tuşlarıyla değiştirin. VoltioVolt değişiklikleri hemen gösterir.

Tuşlar:
  Yukarı, Aşağı              Menü öğesini seç
  Sol, Sağ                   Değeri bir adım değiştir
  Page Up, Page Down         Değeri on adım değiştir
  Enter veya Boşluk          Aç/kapa öğesini değiştir, veya bir işlemi çalıştır
  1 - 9                      Bir senaryoyu çalıştır
  R                          Varsayılan değerlere sıfırla
  L                          Tam günlüğü göster veya gizle
  Q veya Esc                 Çık (onay ister)


ENGLISH
-------

This is a test build. It is not for normal use. Use a test computer.

Method 1: "tar xzf VoltioVolt-test-linux-amd64.tar.gz", then "cd VoltioVolt-test-linux-amd64" and "./voltiovolt". No installation is necessary.

Method 2: "sh install.sh". No sudo. The files are copied to ~/.local/opt/voltiovolt. If you agree, ~/.local/share/applications/voltiovolt.desktop is made.

1. Download: open the Releases page on GitHub, select "Test build", download VoltioVolt-test-linux-amd64.tar.gz and SHA256SUMS.txt.
2. Check: "sha256sum --ignore-missing -c SHA256SUMS.txt" must show "OK".
3. Run-time packages (Debian and Ubuntu): "sudo apt install libgtk-4-1 libwebkitgtk-6.0-4".
4. No other program (for example UPSilon 2000) may use the UPS port.
5. If your user cannot open /dev/ttyUSB* or /dev/ttyACM*, run this command one time, then log out and log in: sudo usermod -aG dialout <user name>. VoltioVolt does not run this command.
6. Tests: https://github.com/abuzer1495/voltiovolt/blob/main/docs/user/owner-tests.md
7. Uninstall: "sh uninstall.sh". It asks separately before it deletes the data folder.

UPS simulator
-------------

1. Start ./vlt-simulator in a terminal (in the extracted folder, or after install: ~/.local/opt/voltiovolt/vlt-simulator).
2. In VoltioVolt, open Device and select "Test connection (simulator)".
3. Change the values in the simulator with the arrow keys. Watch VoltioVolt: the values change at once.

Keys:
  Up, Down            Select a menu item
  Left, Right         Change the value by one step
  Page Up, Page Down  Change the value by ten steps
  Enter or Space      Switch an on/off item, or run an action
  1 to 9              Run a scenario
  R                   Reset to the default values
  L                   Show or hide the full log
  Q or Esc            Quit (asks for confirmation)

