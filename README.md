# SiteYönet — Site / Apartman Yönetimi

**Geliştirici: Gökdeniz Kılınç** · DDO mülakat projesi

Çoklu site yönetimi için aidat, tahsilat, gider ve muhasebe uygulaması.

## Kurulum

Docker ve Docker Compose gereklidir.

```bash
git clone https://github.com/gokdeniz001/ddo-case-gokdeniz-kilinc.git
cd ddo-case-gokdeniz-kilinc
cp .env.example .env
docker compose up --build
```

Arayüz: [localhost:3000](http://localhost:3000) · API: [localhost:8080](http://localhost:8080) · Sağlık: [/health](http://localhost:8080/health).

Migrasyon ve seed otomatik çalışır. Temiz kurulum 40 site, 6.000 daire ve Kasım 2023–Ekim 2026 arasında 36 aylık geçmiş üretir. PostgreSQL yalnızca Compose ağından erişilir. Mevcut veri otomatik silinmez.

## Mimari ve seçimler

- **Go 1.22, chi ve pgx:** standart HTTP/context yapısı ve PostgreSQL işlemleri üzerinde doğrudan kontrol.
- **PostgreSQL 16:** veri kısıtları, tam sayı kuruş tutarları ve site bazlı Row Level Security (RLS).
- **React 18, TypeScript ve Vite:** ortak ekran bileşenleri, rol bazlı arayüz ve derleme sırasında tip kontrolü.
- **Docker Compose:** ayrı API, frontend, veritabanı ve simülatör servisleri; otomatik kurulum ve sağlık kontrolleri.

Korunan istekler, kısıtlı veritabanı rolü ve site/kullanıcı bağlamı ile tek transaction içinde çalışır. Site bazlı yazma kilidi eşzamanlı değişiklikleri sıralar; idempotans yanıtı ve denetim kaydı iş işlemiyle birlikte kaydedilir. Mimari kararların alternatifleri ve sonuçları: [DECISIONS.md](DECISIONS.md).

## Demo hesapları

Tüm hesapların parolası: `password123`. Demo arayüzü yönetici hesabıyla açılır; sol menüden rol, üst çubuktan yetkili olunan site değiştirilebilir.

| Rol | E-posta |
|---|---|
| Yönetici | yonetici@siteyonet.com |
| Muhasebe | muhasebe@siteyonet.com |
| Denetim kurulu | denetim@siteyonet.com |
| Daire maliki | malik@siteyonet.com |
| Kiracı | kiraci@siteyonet.com |
| Süper yönetici | admin@siteyonet.com |

## Test ve doğrulama

Sistem hazır olduktan sonra:

```bash
make verify
# make olmayan sistemlerde:
docker compose --profile verify run --build --rm verify

make test       # PostgreSQL entegrasyon ve birim testleri
make load       # k6 yük testi
make reset-demo # Verileri silerek ilk seed durumuna döndürür
```

38 otomatik testin 22'si gerçek PostgreSQL entegrasyon testidir. Doğrulayıcı ayrıca F-1–F-7, T-1–T-4, sağlık ve yetki kontrollerini PASS/FAIL tablosunda gösterir; hatada sıfırdan farklı kodla çıkar. Doğrulama demo verisine test kayıtları ekler.

- [TESTING.md](TESTING.md): özellik bazında beklenen sonuçlar, kanıt komutları ve elle deneme senaryoları.
- [requests.http](requests.http) · [OpenAPI](backend/docs/openapi.yaml): API kullanım örnekleri ve uç noktalar.
- [Test çıktısı](audit/final-go-tests.txt) · [API doğrulama çıktısı](audit/final-verify.txt) · [GitHub Actions](https://github.com/gokdeniz001/ddo-case-gokdeniz-kilinc/actions): test ve CI kanıtları.

## Performans

Yerel Docker ortamında 40 site, 6.000 daire ve 214.560 borç kaydıyla ölçüldü. Tam seed yaklaşık 78 saniye sürdü; imaj indirme ve derleme bu süreye dahil değildir. Boş Docker önbelleğinde 5 dakikalık kurulum hedefi ayrıca ölçülmedi.

60 saniyelik, en fazla 50 kullanıcılı k6 sağlık/borç/dekont senaryosunda sayfalama öncesi p95 **668,12 ms**, sonrası **32,12 ms**; HTTP hata oranı **%0**, kontrol başarısı **%100**. p95 <250 ms eşiği değiştirilmedi. Ham sonuçlar: [önce](audit/load-before-pagination.json), [sonra](audit/load-after-pagination.json).

## Doğrulama Soruları

### 1. En zor görev ve alternatifler

T-3 veri izolasyonu: RLS politikaları mevcut olsa da süper kullanıcı bağlantısı bunları atlıyordu. Yalnızca uygulamada WHERE filtreleri kullanmak yerine kısıtlı SQL rolü ve istek boyunca tek transaction seçildi. Ödeme yarışları için satır kilidi veya serializable/retry yerine site bazlı yazma kilidi kullanıldı; bunun bedeli aynı siteye yazmaların sıralanmasıdır.

### 2. Bilinen sınırlamalar

- Aynı ayda bir malik devri desteklenir; daha fazla devir dağıtımı hata verir.
- Tahakkuk geri alma yalnızca açık dönemde, ödeme almamış ve yevmiyeye bağlı kayıtlarda çalışır. Ödenmiş/kapalı tahakkukun borç ve avans düzeltme akışı tamamlanmadı; muhasebe ters fişi açık döneme yazılabilir.
- Fazla ödeme avans olarak tutulur; sonraki tahakkuka otomatik uygulanmaz. Bütçe site bazındadır, dönem bazlı versiyonlanmaz.
- CSV eşleşmesi tahsilat önerisidir; ödeme yönetici tarafından kaydedilir. Mutabakat yalnızca içe aktarılmış ekstrelere dayanır.
- Aynı seed iş verisini tekrar üretir; UUID, parola salt'ı ve oluşturulma zamanları birebir aynı değildir.

### 3. İki dizin ve ölçüm farkı

| Dizin ve sorgu | Zorlanan sıralı tarama | Normal planner |
|---|---:|---:|
| `debts_site_due_id`: sitenin borçları, `(due_date,id)` sırası, LIMIT 500 | 501,273 ms | 4,597 ms |
| `idx_debts_unit_status`: dairenin ödenmemiş borçları | 298,373 ms | 0,867 ms |

`EXPLAIN (ANALYZE)` çıktısından ilgili satırlar:

```text
Index Scan using debts_site_due_id on "fy-debts"
Execution Time: 4.597 ms
Bitmap Index Scan on idx_debts_unit_status
Execution Time: 0.867 ms
```

[Ölçüm SQL'i](audit/explain.sql) · [Tam plan çıktısı](audit/explain.txt). RLS rolü aktiftir. Karşılaştırmada dizinler fiziksel olarak kaldırılmadı; index/bitmap taramaları kapatılarak sıralı tarama zorlandı. Önbellek ve eşzamanlı yük süreleri etkileyebilir.

### 4. Düzeltilen yapay zekâ çıktısı

**Önce:** T-4 doğrulayıcısı hata durumunda bile `return true` döndürerek başarısız işlemi PASS gösteriyordu.

**Sonra:** Borçsuz daire için belge oluşturulur ve `validity_status` kontrol edilir; borçlu daire için belge talebinin reddi ayrıca sınanır. Hatalar FAIL ve sıfırdan farklı çıkış kodu üretir. Böylece yalnızca isteğin gönderilmesi değil, beklenen iş sonucu doğrulanır.

### 5. Tahakkuk motorunun doğruluk kanıtı

```bash
docker compose --profile verify run --build --rm verify
# Yalnızca ilgili testler için, DATABASE_URL ayarlı backend klasöründe:
go test -v ./tests/... -run 'TestT1|TestIntegration_Accrual|TestIntegration_Ownership'
```

[Test çıktısından](audit/final-go-tests.txt):

```text
--- PASS: TestT1_LargestRemainderMethod_CentExactEqual (0.00s)
--- PASS: TestT1_LargestRemainderMethod_CentExactLandShare (0.00s)
--- PASS: TestT1_ExemptUnitsExcluded (0.00s)
--- PASS: TestT1_MidPeriodOwnershipTransfer_Prorata (0.00s)
--- PASS: TestT1_DistributionType_AreaM2 (0.00s)
--- PASS: TestT1_DistributionType_ResidentCount (0.00s)
--- PASS: TestIntegration_AccrualCentExactAPI (0.20s)
--- PASS: TestIntegration_OwnershipDaySplitAPI (0.25s)
--- PASS: TestIntegration_AccrualReversalLedger (0.27s)
```

Testler dört dağıtım türünü, muafiyeti ve toplamın tam kuruş eşitliğini denetler. HTTP testi 125.433 kuruşluk tahakkukun veritabanına yazılmış borçlarını toplar. Malik devri testi 31 günlük ayda 3.100 kuruşun eski malike 1.000, yeni malike 2.100 kuruş dağıtıldığını doğrular.
