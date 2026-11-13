VoltioVolt - TEST BUILD / TEST SÜRÜMÜ
=====================================

TÜRKÇE
------

Bu bir test sürümüdür. Normal kullanım için değildir. Bir test bilgisayarı kullanın.

Yöntem 1: Zip dosyasındaki tüm dosyaları bir klasöre çıkarın. VoltioVolt.exe dosyasına çift tıklayın. Kurulum gerekmez. Yönetici hakları gerekmez.

Yöntem 2: install.cmd dosyasını çalıştırın. Yönetici hakları gerekmez. Dosyalar "%LOCALAPPDATA%\Programs\VoltioVolt" klasörüne kopyalanır. "Make a Start menu shortcut? (Y/N)" sorusunda Y derseniz, kullanıcı Başlat menüsünde bir kısayol yapılır. VoltioVolt.exe başlar.

1. İndirme: GitHub'da Releases sayfasını açın, "Test build" sürümünü seçin, VoltioVolt-test-windows-amd64.zip ve SHA256SUMS.txt dosyalarını indirin.
2. Kontrol: PowerShell'de "Get-FileHash VoltioVolt-test-windows-amd64.zip -Algorithm SHA256" komutunu çalıştırın. Değer, SHA256SUMS.txt dosyasındaki değerle aynı olmalıdır.
3. SmartScreen: Windows "Windows bilgisayarınızı korudu" gösterirse "Ek bilgi", sonra "Yine de çalıştır" seçin. Dosyalar imzalı değildir.
4. UPSilon 2000 programını ve RupsMon hizmetini durdurun. UPS portunu aynı anda yalnızca bir program kullanabilir.
5. WebView2 gereklidir (çoğu Windows 10 ve 11 bilgisayarında vardır).
6. Eski test (VLT-IS-007) kuruluysa, bir kez remove-old-test-service.cmd dosyasını çalıştırın (yönetici hakları gerekir).
7. Testler: https://github.com/abuzer1495/voltiovolt/blob/main/docs/user/owner-tests.md
8. Kaldırma: uninstall.cmd dosyasını çalıştırın. Veri klasörü silinmeden önce ayrı bir soru sorulur.

UPS simulator
-------------

1. vlt-simulator.exe dosyasını başlatın (klasörde çift tıklayın veya bir cmd penceresinde çalıştırın).
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

Method 1: extract all files of the zip file to a folder. Double-click VoltioVolt.exe. No installation is necessary. No administrator rights are necessary.

Method 2: run install.cmd. No administrator rights are necessary. The files are copied to "%LOCALAPPDATA%\Programs\VoltioVolt". If you answer Y to "Make a Start menu shortcut? (Y/N)", a shortcut is made in the Start menu folder of the user. VoltioVolt.exe starts.

1. Download: open the Releases page on GitHub, select "Test build", download VoltioVolt-test-windows-amd64.zip and SHA256SUMS.txt.
2. Check: in PowerShell, run "Get-FileHash VoltioVolt-test-windows-amd64.zip -Algorithm SHA256". The value must be the same as the value in SHA256SUMS.txt.
3. SmartScreen: if Windows shows "Windows protected your PC", select "More info", then "Run anyway". The files are not signed.
4. Stop UPSilon 2000 and the RupsMon service. Only one program can use the UPS port at a time.
5. WebView2 is necessary (installed on most Windows 10 and 11 computers).
6. If the old test (VLT-IS-007) was installed, run remove-old-test-service.cmd one time (administrator rights are necessary).
7. Tests: https://github.com/abuzer1495/voltiovolt/blob/main/docs/user/owner-tests.md
8. Uninstall: run uninstall.cmd. It asks separately before it deletes the data folder.

UPS simulator
-------------

1. Start vlt-simulator.exe (double-click it in the folder, or run it in a cmd window).
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

