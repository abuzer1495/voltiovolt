# Interface terms

VLT-IS-012 TASK 03-00-00-002 (R-18, PR-78-2). All interface text in `web/i18n/en.json` and `web/i18n/tr.json` uses these terms, and the Turkish text of vlt-simulator uses them where it shows the same value. The test `web/test/terms.test.ts` fails if `tr.json` contains a form of the deny list.

| English term | Turkish term | Where it is used |
|--------------|--------------|------------------|
| Simple mode | Basit mod | Mode button, View menu ("&Basit mod"), wizard step "Interface", General screen |
| Expert mode | Uzman mod | Mode button, View menu ("&Uzman mod"), wizard step "Interface", General screen |
| Setup wizard | Kurulum sihirbazı | Wizard title, first-start dialog, General screen, Tools menu |
| Shutdown Policy | Kapatma politikası | Navigation, Shutdown Policy screen, wizard step, Dashboard, Simple screen |
| Countdown | Geri sayım | Countdown banner, Shutdown Policy fields, Events |
| Mains power | Şebeke | Dashboard mode, tray tooltip, Events, health text, simulator menu |
| On battery | Aküde | Dashboard mode, tray tooltip, policy triggers and help texts |
| Battery charge | Akü doluluğu | Dashboard values, History, policy trigger help, simulator |
| Runtime | Kalan süre | Dashboard values, tray tooltip, policy trigger, health text, simulator |
| Load | Yük | Dashboard values, History, Simple chart, simulator |
| Input voltage | Giriş gerilimi | Dashboard values, History, Simple chart, simulator |
| Output voltage | Çıkış gerilimi | Dashboard values, History, simulator |
| Test connection | Test bağlantısı | Device screen, wizard step "UPS port: select", header, banner |
| Interface size | Arayüz boyutu | View menu, General screen, wizard step "Interface", Simple settings |
| Lock, Unlock | Kilitle, Kilidi aç | Header, tray menu, lock screen |
| Event | Olay | Events screen, Dashboard, Simple screen |
| Notification | Bildirim | Notifications screen, wizard |
| Battery replace | Akü değişimi | Event BATTERY_REPLACE, Simple health, Device screen (VLT-IS-012 TASK 03-00-00-004) |

Deny list for `tr.json` (other forms of the terms above): the word "kip" in all forms ("kip", "kipi", "Uzman kipi", "Basit kipi"), "Uzman kip", "Basit kip", "Kapatma Politikası", "Şebeke elektriği", "pil gücü", "pil şarjı", "çalışma süresi".
