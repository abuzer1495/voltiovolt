# Sürüm notları

Bu dosya [CHANGELOG.md](../../CHANGELOG.md) dosyasının Türkçe kopyasıdır (VLT-IS-012 TASK 03-00-00-007).

## [Unreleased]

### Eklenenler

- Yerel uygulamalar için Gmail OAuth 2.0 (RFC 8252, SEC-045): PKCE S256, state denetimi ve rastgele portlu 127.0.0.1 geri dönüşü. İstenen tek kapsam `https://www.googleapis.com/auth/gmail.send` (Gmail API ile gönderim). SMTP XOAUTH2 kullanılmaz; çünkü `https://mail.google.com/` (tam posta kutusu erişimi) gerekir. Yenileme belirteci yalnızca Kasada tutulur. Erişim belirteci bellekte kalır. Grafik arayüz alanları yalnız yazılır. Hesap kaldırıldığında belirteç Google'da iptal edilir ve Kasa sırları silinir. Uygulama parolası ile SMTP kullanılmaya devam eder.
- `screenshots` CI işi: Demo kipinde arayüzün İngilizce ve Türkçe ekran görüntülerini alır ve `screenshots.zip` artifaktı olarak yükler. Görüntüler depoya eklenmez (SEC-087).

### Düzeltilenler

- Windows: profil "DTR ve RTS değiştirilmez" iken port açılışında DTR ve RTS artık yüksek çekilmez (VLT-RPT-006). 0.9.0-rc.1 sürüm notlarındaki bilinen sorun o sürüm için geçerliliğini korur. FIELD: Owner, Referans UPS üzerinde doğrulamalıdır (T-01).

## [0.9.0-rc.1] – 2026-11-20

### Eklenenler

- VoltioVolt, Windows ve Linux (x86_64) için taşınabilir bir uygulama: tepsi simgesi olan tek bir program. İşletim sisteminde değişiklik yapmaz. Verilerini programın yanındaki `data` klasöründe tutar.
- USB seri port (COM portu) veya USB HID ile UPS bağlantısı. VoltioVolt Megatec, Voltronic QS veya Voltronic protokolünü bulur. Referans UPS için "Tunçmatik Newtech Pro II (Megatec)" profili hazırdır.
- Bilinen bir USB-seri yongası olan port "Muhtemel UPS" işaretini alır. VoltioVolt, seçmediğiniz bir porta hiçbir şey göndermez.
- Canlı değerleri olan gösterge paneli, Geçmiş grafikleri, Olaylar ve akü doluluğu tahmini.
- Kapatma politikası: bilgisayar, aküde belirli bir süreden sonra, kalan süre azaldığında veya akü doluluğu düştüğünde kapanır ya da hazırda bekler. İptal düğmesi olan bir geri sayım ve etkinleştirmeden önce bir Simülasyon vardır. İsteğe bağlı: UPS çıkışını kapatır ve belirli bir süre sonra yeniden açar.
- Masaüstünde, Telegram'da ve e-posta ile (Gmail veya SMTP) Bildirimler.
- Ayarlar için isteğe bağlı PIN ve "PIN'i unuttum". Parolalar ve belirteçler şifreli olarak tutulur.
- Kurulum sihirbazı, Basit mod ve Uzman mod, arayüz boyutu, Türkçe ve İngilizce.
- vlt-simulator simülatörü ile Demo modu ve Test bağlantısı: VoltioVolt'u UPS olmadan deneyin.
- Akü değişimi uyarısı: başarısız bir akü testinden veya yazdığınız akü takılma tarihinden.
- Her alan için yardım ve resmi belgelere bağlantılar.

### Değişenler

- Varsayılan paket taşınabilirdir. Hizmet modu (vltd) yalnızca kaynak kodunda kalır.
- Arayüz için tek bir Türkçe terim listesi, örneğin "Basit mod", "Uzman mod" ve "Kapatma politikası".
- Sürüm; Hakkında penceresinde, tepsi ipucunda, `--version` çıktısında ve sürüm adayında pencere başlığında görünür.

### Düzeltilenler

- İlk PIN belirlendikten sonra PIN ile kilit açma çalışır.
- Geçmiş grafikleri canlı güncellenir ve seçtiğiniz serileri korur.
- Sihirbazın her adımı kaydırma çubuğu olmadan sığar. İletişim kutusunun arkasındaki pencere kaymaz. Hiçbir metin kesilmez.
- Taşınabilir modda üst bölüm UPS bağlantısını gösterir, örneğin "UPS'e bağlı (COM3)"; "vltd'ye bağlı" göstermez.

### Bilinen sorunlar

- T-09 ve T-12 kapatma testleri henüz gerçek donanımda yapılmadı.
- Referans UPS'in kaydı (T-01) henüz yapılmadı. Q1, F ve I dışındaki komutlar Referans UPS'te doğrulanmadı.
- Dosyalar imzalı olmadığı için Windows SmartScreen bir uyarı gösterir.
- Windows'ta port açıldığında DTR ve RTS açıktır (VLT-RPT-006).
