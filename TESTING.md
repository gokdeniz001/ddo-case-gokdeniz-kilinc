# SiteYönet — Test, Doğrulama ve Değerlendirme Kılavuzu

> Bu belge, değerlendirici ve ekip liderinin sistemi en hızlı şekilde denemesi, tüm zorlu görevlerin çalıştığını doğrulaması ve kanıt komutlarını çalıştırması için hazırlanmıştır.

---

## 1. Demo Hesapları (Tüm Roller)

Tüm hesapların varsayılan parolası: **`password123`**

| Rol | E-Posta | Açıklama ve İzin Kapsamı |
|---|---|---|
| **Süper Admin** | `admin@siteyonet.com` | Sistem geneli tam yetki |
| **Site Yöneticisi** | `yonetici@siteyonet.com` | Site, blok, daire, tahakkuk, tahsilat, gider ve karar yönetimi |
| **Muhasebe** | `muhasebe@siteyonet.com` | Yevmiye kayıtları, mizan, gelir tablosu, dönem kapanışı |
| **Denetim Kurulu** | `denetim@siteyonet.com` | Tüm mali hareketleri **salt-okunur** inceleme (yazma yetkisi yok) |
| **Daire Maliki** | `malik@siteyonet.com` | Yalnızca kendi dairesinin borç/ödemelerini görme, bakım talebi |
| **Kiracı** | `kiraci@siteyonet.com` | Oturduğu daireye ait işletme aidatı borçlarını görme |

*(Not: Herhangi bir hesaba frontend arayüzündeki sol altta yer alan "Hızlı Demo Rol Değiştir" butonlarıyla tek tıkla geçiş yapabilirsiniz.)*

---

## 2. Tek Komutla Otomatik Doğrulama (Zorunlu)

Sistem ayaktayken repo kökünde tek komutla tüm özellikleri ve zorlu görevleri test edin:

```bash
make verify
# veya
docker compose --profile verify run --rm verify
```

Bu komut aşağıdaki tüm kimlikleri sırayla dener ve terminale `PASS` / `FAIL` tablosu basar.

---

## 3. Doğrulama Kimlikleri Tablosu

| Kimlik | Özellik / Görev | Nasıl Denenir? | Beklenen Sonuç |
|---|---|---|---|
| **`HEALTH`** | Genel Sağlık Kontrolü | `GET http://localhost:8080/health` | `HTTP 200` + `{"status":"healthy","database":"connected"}` |
| **`DDO-HZ`** | DDO Özel Sağlık Ucu | `GET http://localhost:8080/_hz/0c7c` | `HTTP 200 OK` (Body: `OK 0c7c`) |
| **`DDO-HDR`** | DDO Özel HTTP Başlığı | Herhangi bir API yanıtı | Her HTTP yanıtında `X-DDO-Build: 0c7cc1` başlığı var |
| **`AUTH-FAIL`**| Hatalı Parola Girişi | `POST /api/v1/auth/login` (yanlış parola) | `HTTP 401 Unauthorized` |
| **`AUTH-RBAC`**| Yetki İhlali Senaryosu | Daire sakini tokenı ile `POST /api/v1/dues-definitions` | `HTTP 403 Forbidden` |
| **`F-1`** | Site, Blok ve Daire | `GET /api/v1/units` (Yönetici) | Bloklar, kat mülkiyeti arsa payları ve sakinler listelenir |
| **`F-2`** | Aidat Tanımı & Tahakkuk | `GET /api/v1/dues-definitions` | Tanımlı aidat şablonları listelenir |
| **`F-3`** | Tahsilat & Borç Takibi | `GET /api/v1/debts` | Borç kalemleri, ödenen ve kalan tutarlar listelenir |
| **`F-4`** | Gider Yönetimi | `GET /api/v1/expenses` | Elektrik, su, asansör faturaları ve tedarikçiler listelenir |
| **`F-5`** | Duyuru, Karar & Bakım | `GET /api/v1/announcements` & `/maintenance` | Duyurular ve sakin arıza talepleri listelenir |
| **`F-6`** | Muhasebe Defteri | `GET /api/v1/periods` & `/journal-entries` | Mali dönemler ve çift taraflı yevmiye fişleri gelir |
| **`F-7`** | Raporlar & Belge | `GET /api/v1/reports/summary` | Tahsilat oranı, net nakit akışı ve kasa durumu gelir |
| **`T-1`** | **Tahakkuk Motoru** | `POST /api/v1/accruals` (küsuratlı tutar) | En Büyük Kalan Yöntemi ile kuruşu kuruşuna tam eşit dağıtılır |
| **`T-2`** | **Tahsilat & Dekont** | `POST /api/v1/bank-statements/match` | Hatalı açıklamadan daire Levenshtein ile bulunur, FIFO tahsis edilir |
| **`T-3`** | **Veri İzolasyonu (RLS)**| Sakin/Denetçi ile `GET /api/v1/debts` | Daire sakini başkasını göremez; Denetçi INSERT yapamaz (403) |
| **`T-4`** | **Muhasebe & Belge** | `GET /api/v1/certificates/verify/:code` | Borcu olan daireye belge verilmez; verilen belge SHA-256 ile doğrulanır |

---

## 4. Zorlu Görevler İçin Kanıt Komutları

Her zorlu görevin matematiksel ve mimari olarak çalıştığını kanıtlayan komutlar:

### Görev 1 (T-1) Kanıt Komutu: Kuruşu Kuruşuna Dağıtım ve Prorata
```bash
cd backend && go test -v -run "TestT1" ./tests/...
```
*Kanıt:* 3 daireye 10.000 TL dağıtıldığında toplamın tam 1.000.000 kuruş olduğunu, muaf dairelerin dışlandığını ve dönem ortası el değiştirmede gün bazlı paylaştırıldığını kanıtlar.

### Görev 2 (T-2) Kanıt Komutu: Waterfall Mahsup & Bulanık Eşleme
```bash
cd backend && go test -v -run "TestT2" ./tests/...
```
*Kanıt:* Kısmi ödemede önce faiz, sonra anapara, sonra en eski borcun ödendiğini; fazla paranın avans alacak yapıldığını ve Levenshtein ile daire eşleşmesini kanıtlar.

### Görev 3 (T-3) Kanıt Komutu: PostgreSQL RLS Çok Kiracılı İzolasyon
```bash
cd backend && go test -v -run "TestT3" ./tests/...
```
*Kanıt:* Daire sakininin komşusunun borcunu göremediğini ve denetçinin `INSERT` denemesinde veritabanı seviyesinde `Permission Denied` aldığını kanıtlar.

### Görev 4 (T-4) Kanıt Komutu: Çift Taraflı Muhasebe & Kriptografik Belge
```bash
cd backend && go test -v -run "TestT4" ./tests/...
```
*Kanıt:* Borç = Alacak dengesi bozuk kaydın reddedildiğini, borcu olan daireye sertifika verilmediğini ve SHA-256 hash bütünlüğünü kanıtlar.

### Eşzamanlılık (Concurrency) Yarış Durumu Kanıt Komutu
```bash
cd backend && go test -v -run "TestConcurrency" ./tests/...
```

---

## 5. Arayüzde Elle Denenecek 3 Dakikalık Senaryo

1. Tarayıcınızda **`http://localhost:3000`** adresini açın.
2. Sol alt panelden **"👑 Yönetici"** rolüne tıklayın.
3. Sol menüden **"Tahakkuk & Dağıtım (T-1)"** sekmesine gidin.
   - Dağıtılacak tutara `15432.17` yazın ve **"Tahakkuku Dağıt"** butonuna basın. Kuruşu kuruşuna denk dağıtıldığını görün.
4. Sol menüden **"Tahsilat & Dekont Eşleme (T-2)"** sekmesine gidin.
   - Açıklama kutusuna `A BLOK NO 12 AHMET YILMAZ` yazıp **"Daireyi Eşleştir"**e basın. Sistemin daireyi bulduğunu görün.
5. Sol alt panelden **"🔍 Denetçi"** rolüne geçin.
   - Tüm finansal raporları ve yevmiye kayıtlarını görebildiğinizi, ancak yeni gider veya tahakkuk ekleme yetkinizin olmadığını teyit edin.
6. Sol menüden **"'Borcu Yoktur' Belgesi (T-4)"** sekmesine gidin.
   - Borcu 0 olan bir daire seçip **"Belgeyi İmzala ve Üret"** butonuna basın. Üretilen doğrulama kodunu yandaki kutuya yapıştırıp **"Doğrula"** butonuna basın. SHA-256 kriptografik yeşil onay rozetini görün.
