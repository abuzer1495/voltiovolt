VoltioVolt - TEST BUILD / TEST SÜRÜMÜ
=====================================

TÜRKÇE
------

Bu bir test sürümüdür. Normal kullanım için değildir. Bir test bilgisayarı kullanın.

1. İndirme: GitHub'da Releases sayfasını açın, "Test build" sürümünü seçin, VoltioVolt-test-linux-amd64.tar.gz ve SHA256SUMS.txt dosyalarını indirin.
2. Kontrol: "sha256sum --ignore-missing -c SHA256SUMS.txt" komutu "OK" yazmalıdır.
3. Çıkarma: "tar xzf VoltioVolt-test-linux-amd64.tar.gz", sonra "cd VoltioVolt-test-linux-amd64".
4. Çalışma zamanı paketleri (Debian ve Ubuntu): "sudo apt install libgtk-4-1 libwebkitgtk-6.0-4". systemd 250 veya sonrası gereklidir.
5. UPS portunu başka bir program (örneğin UPSilon 2000) kullanmamalıdır.
6. Kurulum: "sudo sh install-test.sh". Dosyalar /usr/local/lib/voltiovolt-test/ klasörüne kopyalanır ve vltd başlar.
7. Oturumu kapatıp yeniden açın (vlt grubu). VLT GUI'yi başlatın: /usr/local/lib/voltiovolt-test/vlt-gui
8. Testler: https://github.com/abuzer1495/voltiovolt/blob/main/docs/user/owner-tests.md
9. Kaldırma: "sudo sh uninstall-test.sh". Veriler silinmeden önce soru sorulur.

ENGLISH
-------

This is a test build. It is not for normal use. Use a test computer.

1. Download: open the Releases page on GitHub, select "Test build", download VoltioVolt-test-linux-amd64.tar.gz and SHA256SUMS.txt.
2. Check: "sha256sum --ignore-missing -c SHA256SUMS.txt" must show "OK".
3. Extract: "tar xzf VoltioVolt-test-linux-amd64.tar.gz", then "cd VoltioVolt-test-linux-amd64".
4. Run-time packages (Debian and Ubuntu): "sudo apt install libgtk-4-1 libwebkitgtk-6.0-4". systemd 250 or later is necessary.
5. No other program (for example UPSilon 2000) may use the UPS port.
6. Install: "sudo sh install-test.sh". The files are copied to /usr/local/lib/voltiovolt-test/ and vltd starts.
7. Log out and log in again (group vlt). Start the VLT GUI: /usr/local/lib/voltiovolt-test/vlt-gui
8. Tests: https://github.com/abuzer1495/voltiovolt/blob/main/docs/user/owner-tests.md
9. Uninstall: "sudo sh uninstall-test.sh". It asks before it deletes the data.
