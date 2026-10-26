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
| **`AUTH-4`**| Hatalı Parola Girişi | `POST /api/v1/auth/login` (yanlış parola) | `HTTP 401 Unauthorized` |
| **`AUTH-5`**| Yetki İhlali Senaryosu | Daire sakini tokenı ile `POST /api/v1/dues-definitions` | `HTTP 403 Forbidden` |
| **`F-1`** | Site, Blok ve Daire | `GET /api/v1/units` (Yönetici) | Bloklar, kat mülkiyeti arsa payları ve sakinler listelenir |
| **`F-2`** | Aidat Tanımı & Tahakkuk | `GET /api/v1/dues-definitions` | Tanımlı aidat şablonları listelenir |
| **`F-3`** | Tahsilat & Borç Takibi | `GET /api/v1/debts` | Borç kalemleri, ödenen ve kalan tutarlar listelenir |
| **`F-4`** | Gider Yönetimi | `GET /api/v1/expenses` | Elektrik, su, asansör faturaları ve tedarikçiler listelenir |
| **`F-5`** | Duyuru, Karar & Bakım | `GET /api/v1/announcements` & `/maintenance` | Duyurular ve sakin arıza talepleri listelenir |
| **`F-6`** | Muhasebe Defteri | `GET /api/v1/periods` & `/journal-entries` | Mali dönemler ve çift taraflı yevmiye fişleri gelir |
| **`F-7`** | Raporlar & Belge | `GET /api/v1/reports/summary` | Tahsilat oranı, net nakit akışı ve kasa durumu gelir |
| **`T-1`** | **Tahakkuk Motoru** | `POST /api/v1/accruals` (küsuratlı tutar) | En Büyük Kalan Yöntemi ile kuruşu kuruşuna tam eşit dağıtılır |
| **`T-2`** | **Tahsilat & Dekont** | `POST /api/v1/bank-statements/match` | Açıklamadan daire önerilir; ayrı tahsilat isteği faiz → anapara / FIFO tahsis eder |
| **`T-3`** | **Veri İzolasyonu (RLS)**| Sakin/Denetçi ile `GET /api/v1/debts` | Daire sakini başkasını göremez; Denetçi INSERT yapamaz (403) |
| **`T-4`** | **Muhasebe & Belge** | `GET /api/v1/certificates/verify/:code` | Borcu olan daireye belge verilmez; verilen belge HMAC-SHA256 ile doğrulanır |

---

## 4. Zorlu Görevler İçin Kanıt Komutları

Aşağıdaki Go komutları PostgreSQL bağlantısını içeren `DATABASE_URL` ile çalıştırılır. Go kurulu değilse `make verify` aynı testleri Docker içinde çalıştırır. Kapsam sınırları README’de belirtilmiştir.

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
cd backend && go test -v -run "TestIntegration_.*Privacy|TestIntegration_RLS|TestIntegration_Auditor|TestIntegration_CrossSite|TestIntegration_MultiSite" ./tests/...
```
*Kanıt:* Daire sakininin komşusunun borcunu göremediğini ve denetçinin `INSERT` denemesinde veritabanı seviyesinde `Permission Denied` aldığını kanıtlar.

### Görev 4 (T-4) Kanıt Komutu: Çift Taraflı Muhasebe & Kriptografik Belge
```bash
cd backend && go test -v -run "TestT4|TestIntegration_Closed|TestIntegration_Certificate" ./tests/...
```
*Kanıt:* Borç = Alacak dengesi bozuk kaydın reddedildiğini, borcu olan daireye sertifika verilmediğini ve HMAC-SHA256 imza bütünlüğünü kanıtlar.

### Eşzamanlılık (Concurrency) Yarış Durumu Kanıt Komutu
```bash
cd backend && go test -v -race -run "TestIntegration_ConcurrentPayments|TestIntegration_IdempotencyConcurrentSameKey" ./tests/...
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
   - Borcu 0 olan bir daire seçip **"Borç Kontrolü Yap & Belge Üret"** butonuna basın. Üretilen doğrulama kodunu yandaki kutuya yapıştırıp **"Doğrula"** butonuna basın. Belgenin geçerli olduğuna ilişkin sonucu görün. Belge oluşturmak için yönetici rolüne dönün; borçlu dairede talebin reddedilmesi beklenir.


## 26 Ekim 2026 teslim öncesi doğrulama güncellemesi

- `docker compose --profile verify run --build --rm verify`: Go birim/gerçek PostgreSQL entegrasyon testlerini, ardından çalışan HTTP API üzerinde F-1…F-7 / T-1…T-4 kabul senaryolarını çalıştırır. Herhangi bir hata işlemi sıfırdan farklı kodla bitirir. Doğrulama `verify-...` adlı demo kayıtları ekler; temiz başlangıç için `make reset-demo` kullanılır.
- Entegrasyon testleri boş veya eksik veride SKIP etmez. Her test kendi site, kullanıcı, daire ve borç verisini oluşturur, sonunda temizler.
- T-1: `go test -v ./tests/... -run 'TestT1|TestIntegration_Accrual|TestIntegration_Ownership'`; tam kuruş dağıtımı, dört dağıtım türü, muafiyet, 31 günlük ayda 10/21 günlük malik değişimi.
- T-2: `go test -v -race ./tests/... -run 'TestT2|TestIntegration_.*Payment|TestIntegration_.*Idempotency|TestIntegration_Restructure'`; kısmi ödeme, fazla ödeme, faiz/anapara sırası, gerçek paralel tahsilat, aynı anahtarın eşzamanlı tekrarı, yapılandırma.
- T-3: `go test -v ./tests/... -run 'TestIntegration_.*Privacy|TestIntegration_RLS|TestIntegration_Auditor|TestIntegration_CrossSite|TestIntegration_MultiSite'`; dolu iki ayrı site, doğrudan kısıtlı SQL rolü, sakin mahremiyeti, denetçi yazma engeli, yetkisiz site geçişi.
- T-4: `go test -v ./tests/... -run 'TestT4|TestIntegration_Closed|TestIntegration_Certificate'`; çift taraflı kayıt, kapalı döneme yazma engeli, borçlu dairenin belge talebinin reddi ve gerçek belge imza kontrolü. F-6 API senaryosu ayrıca fiş/ters fiş oluşturur ve satır toplamlarını okur.
- Go komutları `backend/` içinde, PostgreSQL'e erişen `DATABASE_URL` ile çalıştırılır. Go yüklü değilse tek compose doğrulama komutu yeterlidir.
- Yük: `make load` veya `docker run --rm --network siteyonet_siteyonet-net -e API_URL=http://backend:8080 --mount type=bind,source=<repo>/k6,target=/scripts,readonly grafana/k6:0.49.0 run /scripts/load-test.js`. Docker ağ adı compose proje adına bağlıdır.
- `audit/load-before-pagination.json`, `audit/load-after-pagination.json` ve `audit/explain.txt` gerçek ölçümlerdir. Yerel ölçümler bulut veya soğuk önbellek garantisi değildir.

### İlave elle deneme

1. Yönetici → Duyuru, Karar & Bakım: duyuru/karar oluştur, bakım kaydını ilerlet. Denetçi rolünde aynı ekranda yeni kayıt formu görünmemeli.
2. Temiz tam seed sonrası yönetici üst çubuktaki site seçicisiyle ikinci siteye geçer; yalnızca seçilen sitenin daireleri görünür. Üye olmayan siteye `POST /auth/site` 403 döner.
3. Malik ve kiracı rollerinde yönetim menüleri gizlenir; yalnızca bağlı olunan dairelerin borçları görünür. Önceki demo verisi yeniden seed edilmediyse örnek hesabın daire bağlantıları eski kalabilir.
4. `requests.http`: yeni örneklerde önce ilgili named request'i çalıştırın. `createdBlock`, `createdUnit`, `createdMaintenance`, `createdJournal` yanıtları bir sonraki isteğin kimliğini sağlar. Dönem kapanışı yalnızca koleksiyonun oluşturduğu 2999 deneme dönemini hedefler; tekrar oluşturmak için tarihleri değiştirin.
5. Borç sayfalama: `GET /debts?limit=1`, sonra son kaydın `due_date` tarih kısmı ve `id` değeri ile `GET /debts?limit=1&after=YYYY-MM-DD,UUID`. Kayıt tekrarı olmamalı.

Bilinen kapsam sınırları ve teslim değerlendirmesi için [DELIVERY_AUDIT.md](DELIVERY_AUDIT.md).
