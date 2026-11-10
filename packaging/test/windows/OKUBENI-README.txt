VoltioVolt - TEST BUILD / TEST SÜRÜMÜ
=====================================

TÜRKÇE
------

Bu bir test sürümüdür. Normal kullanım için değildir. Bir test bilgisayarı kullanın.

1. İndirme: GitHub'da Releases sayfasını açın, "Test build" sürümünü seçin, VoltioVolt-test-windows-amd64.zip ve SHA256SUMS.txt dosyalarını indirin.
2. Kontrol: PowerShell'de "Get-FileHash VoltioVolt-test-windows-amd64.zip -Algorithm SHA256" komutunu çalıştırın. Değer, SHA256SUMS.txt dosyasındaki değerle aynı olmalıdır.
3. Zip dosyasındaki tüm dosyaları bir klasöre çıkarın.
4. SmartScreen: Windows "Windows bilgisayarınızı korudu" gösterirse "Ek bilgi", sonra "Yine de çalıştır" seçin. Dosyalar imzalı değildir.
5. UPSilon 2000 programını ve RupsMon hizmetini durdurun. UPS portunu aynı anda yalnızca bir program kullanabilir.
6. install-test.cmd dosyasına çift tıklayın. Yönetici haklarını onaylayın (UAC). Dosyalar "C:\Program Files\VoltioVolt-Test\" klasörüne kopyalanır ve vltd hizmeti başlar. "Done" yazısını bekleyin.
7. Oturumu kapatıp yeniden açın. Başlat menüsünden "VoltioVolt (test)" programını başlatın.
8. Testler: https://github.com/abuzer1495/voltiovolt/blob/main/docs/user/owner-tests.md
9. Kaldırma: uninstall-test.cmd dosyasına çift tıklayın. Veriler silinmeden önce soru sorulur.

ENGLISH
-------

This is a test build. It is not for normal use. Use a test computer.

1. Download: open the Releases page on GitHub, select "Test build", download VoltioVolt-test-windows-amd64.zip and SHA256SUMS.txt.
2. Check: in PowerShell, run "Get-FileHash VoltioVolt-test-windows-amd64.zip -Algorithm SHA256". The value must be the same as the value in SHA256SUMS.txt.
3. Extract all files of the zip file to a folder.
4. SmartScreen: if Windows shows "Windows protected your PC", select "More info", then "Run anyway". The files are not signed.
5. Stop UPSilon 2000 and the RupsMon service. Only one program can use the UPS port at a time.
6. Double-click install-test.cmd. Accept the Administrator rights (UAC). The files are copied to "C:\Program Files\VoltioVolt-Test\" and the service vltd starts. Wait for "Done".
7. Sign out and sign in again. Start "VoltioVolt (test)" from the Start menu.
8. Tests: https://github.com/abuzer1495/voltiovolt/blob/main/docs/user/owner-tests.md
9. Uninstall: double-click uninstall-test.cmd. It asks before it deletes the data.
