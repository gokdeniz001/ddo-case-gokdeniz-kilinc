# SiteYönet — Site / Apartman Yönetimi

**Geliştirici: Gökdeniz Kılınç** · DDO mülakat projesi

Go API, React arayüzü ve PostgreSQL 16 ile site aidatı, tahsilat ve muhasebe takibi. Desteklenen akışlar ile sınırlar aşağıda ve [teslim denetiminde](DELIVERY_AUDIT.md) açıklanmıştır.

## Kurulum

```bash
git clone https://github.com/gokdeniz001/ddo-case-gokdeniz-kilinc.git
cd ddo-case-gokdeniz-kilinc
cp .env.example .env
docker compose up --build
```

Frontend: [localhost:3000](http://localhost:3000) · API: [localhost:8080](http://localhost:8080) · Sağlık: [/health](http://localhost:8080/health).
PostgreSQL yalnızca compose ağına açıktır. Backend migrasyonları otomatik çalıştırır; simülatör seed tamamlandığında frontend başlar.

Temiz kurulumun varsayılanı **40 site, 6.000 daire, Kasım 2023–Ekim 2026 arasında 36 ay ve 214.560 borç kaydıdır**. Küçük demo için `.env` içinde `FULL_SIMULATION=false` seçilebilir. Eski bir veri hacmi otomatik silinmez; ölçek değişikliği temiz seed gerektirir.

```bash
make verify
# make bulunmayan sistemlerde:
docker compose --profile verify run --build --rm verify

make test       # Docker içinde gerçek PostgreSQL testleri
make load       # Docker içinde 60 saniyelik k6 testi
make reset-demo # Demo verisini siler ve ilk seed durumuna döndürür
```

Doğrulama demo verisine `verify-...` adlı kayıtlar ekler. Hata varsa sıfırdan farklı kodla biter. Ayrıntılar: [TESTING.md](TESTING.md), [requests.http](requests.http), [OpenAPI](backend/docs/openapi.yaml).

## Mimari ve seçimler

- **Go 1.22 + chi + pgx:** standart HTTP/context desteği ve PostgreSQL işlemlerine doğrudan erişim.
- **PostgreSQL 16:** yabancı anahtar/kısıtlar, dizinler ve veritabanında RLS.
- **React 18 + TypeScript + Vite:** veri ekranlarını bileşenlere ayırmak ve rol geçişlerini sayfa yenilemeden göstermek. Build sırasında TypeScript kontrolü çalışır.
- **Docker Compose:** otomatik migrasyon/seed, healthcheck ve servis bağımlılıkları. API, simülatör, frontend ve doğrulayıcı uygulamaları root olmayan kullanıcılarla çalışır.

Korunan her HTTP isteği aynı PostgreSQL işlemini kullanır. `SET LOCAL ROLE siteyonet_runtime` ve transaction-local site/kullanıcı/rol bağlamı RLS'yi etkinleştirir. Başarı yanıtı yalnızca commit tamamlanırsa gönderilir. Yazmalar site bazlı advisory lock ile sıralanır. Kritik POST isteklerinin idempotans yanıtı iş etkisiyle aynı işlemde tutulur; farklı içerikle anahtar tekrarında 409 döner. Denetim kayıtları aynı işleme eklenir; uygulama rolü bunları değiştiremez veya silemez.

Kararların bağlamı, seçenekleri ve sonuçları: [DECISIONS.md](DECISIONS.md).

## Demo hesapları

Tüm demo hesaplarının parolası `password123`. Arayüz değerlendirmeyi kolaylaştırmak için yönetici hesabıyla açılır ve sol menüden rol değiştirilebilir.

| Rol | E-posta |
|---|---|
| Yönetici | yonetici@siteyonet.com |
| Muhasebe | muhasebe@siteyonet.com |
| Denetim kurulu | denetim@siteyonet.com |
| Daire maliki | malik@siteyonet.com |
| Kiracı | kiraci@siteyonet.com |
| Süper yönetici | admin@siteyonet.com |

Tam temiz seed'de yönetici bütün sitelere üyedir ve üst çubuktan site seçebilir. Malik ve kiracı bir gerçek daireye bağlıdır. Parolalar bcrypt ile özetlenir; erişim JWT'sine ek olarak veritabanında yalnızca özeti tutulan, tek kullanımlık ve dönen refresh token kullanılır.

## Kapsam ve kanıt

| Kimlik | Uygulanan akış / kanıt |
|---|---|
| F-1 | Site/blok/daire listeleri; blok ve daire oluşturma; yetkili çoklu site geçişi |
| F-2 / T-1 | Eşit, arsa payı, m² ve kişi dağıtımı; muafiyet; rasyonel aritmetik ve en büyük kalan; ay içindeki tek malik devrine gün esaslı pay |
| F-3 / T-2 | Gerçek tahsilat formu; faiz → anapara ve FIFO; kısmi/fazla ödeme; kalıcı idempotans; yapılandırma; CSV ekstre içe aktarma ve bulanık daire eşleştirme |
| F-4 | Fatura kaydı, gerçek gider listesi ve tedarikçi borcu yevmiyesi |
| F-5 | Duyuru, karar, bakım oluşturma/listeleme ve bakım durumu ilerletme |
| F-6 / T-4 | Çift taraflı fiş, ters fiş, açık/kapalı dönem ve veritabanı yazma koruması; bütçe karşılaştırması ve mutabakat raporu |
| F-7 | Finansal özet, borçlu daireye belge reddi, borçsuz daireye HMAC-SHA256 ile imzalanmış ve çevrimiçi doğrulanabilir belge |
| T-3 | Dolu iki site üzerinde doğrudan RLS kontrolü, sakin gizliliği, denetçi yazma engeli ve çapraz site kimliklerine yazma engeli |

Banka CSV başlığı: `date,reference,description,amount_cents`. Tutar tam sayı kuruştur. Eşleşme tek başına tahsilat yaratmaz; yönetici daire/tutarı kontrol ederek tahsilatı kaydeder. Temiz seed'de simülatör ayrıca kendi konteynerinin `/app/bank-statements/` klasörüne site bazlı CSV dosyaları yazar.

Fazla ödeme `unallocated_cents` ve 340 avans hesabında tutulur. Yeni tahakkuka otomatik tüketilmesi bu sürümde yoktur. Tahsilat yevmiyesinde anapara 120, gecikme zammı 642, avans 340 hesabına ayrılır. Ödenmemiş gider banka ödemesi sayılmaz.

## Gerçek ölçümler

Yerel Docker ortamı, PostgreSQL 16, 40 site / 6.000 daire / 214.560 başlangıç borcu:

- Tam seed: son ölçüm yaklaşık **78 saniye**. Bu süreye temiz makinede imaj indirme/derleme dahil değildir.
- k6: 60 saniye, en fazla 50 kullanıcı; aynı sağlık/borç/dekont senaryosu. Sayfalama öncesi p95 **668,12 ms**, sonrasında **32,12 ms**; hata oranı **%0**, kontrol başarısı **%100**. Eşik değiştirilmedi: p95 <250 ms.
- Ham ölçümler: [önce](audit/load-before-pagination.json), [sonra](audit/load-after-pagination.json).
- SQL planları: [komut](audit/explain.sql), [tam çıktı](audit/explain.txt). RLS rolü aktiftir. Sıralı tarama karşılaştırması planner'da index/bitmap taramalarının kapatılmasıyla üretilmiştir; dizinler fiziksel olarak silinmemiştir. Önbellek ve eşzamanlı yük süreleri etkiler.

| Dizin / sorgu | Zorlanan sıralı tarama | Normal planner |
|---|---:|---:|
| `debts_site_due_id` / site borçları, `(due_date,id)`, LIMIT 500 | 501,273 ms | 4,597 ms |
| `idx_debts_unit_status` / daireye ait ödenmemiş borçlar | 298,373 ms | 0,867 ms |

Borç API'si varsayılan 500, en fazla 1.000 kayıt döndürür. Son kaydın tarih/kimliğiyle `?limit=500&after=YYYY-MM-DD,UUID` kullanılır. Frontend sayfaları birleştirir.

## Doğrulama Soruları

### 1. En zor görev ve alternatifler

En kritik çalışma T-3 izolasyonunu gerçek isteklerle kanıtlamaktı: yalnızca RLS politikası yazmak yeterli değildi, süper kullanıcı bağlantısı bu politikaları atlıyordu. Yalnızca uygulama WHERE filtreleri yerine kısıtlı SQL rolü ve istek boyunca tek işlem seçildi. Ödeme yarışlarında satır kilidi/serializable-retry seçeneklerine karşı demo için site yazma kilidi tercih edildi; bedeli aynı siteye yazmaların sıralanmasıdır.

### 2. Bilinen sınırlamalar

- Aynı ayda birden fazla malik devri otomatik dağıtılmaz; hata döner.
- Otomatik tahakkuk geri alma yalnızca açık dönemde, ödeme almamış ve yevmiyeye bağlı kayıtlarda desteklenir. Ödeme almış/kapalı dönemdeki tahakkuk için tam borç/avans düzeltme iş akışı yoktur. Muhasebe fişinin ters kaydı açık döneme yazılabilir.
- Avansın sonraki borca otomatik tüketimi ve dönem bazında bütçe versiyonlama yoktur; bütçe site bazındadır.
- Mutabakat raporu kaydedilmiş ekstrelere göre farkı gösterir; gerçek bankayla entegrasyon veya ekstre tamlığı garantisi vermez.
- Aynı seed iş verisini tekrar üretir; UUID, bcrypt salt ve oluşturulma zamanları birebir aynı değildir.

### 3. İki dizin ve ölçüm farkı

Yukarıdaki gerçek planlarda `debts_site_due_id` ilk sayfanın site/tarih/kimlik sırasını, `idx_debts_unit_status` tek dairenin ödenmemiş borç sorgusunu hızlandırdı. Tekrar üretme betiği ve ham planlar `audit/` altındadır; sonuçlar örnek sayı değildir.

### 4. Reddedilen veya düzeltilen yapay zekâ çıktısı

Önceki doğrulayıcı T-4 hata durumunda bile `return true` yapıyordu; bu çıktı reddedildi. Yeni doğrulayıcı gerçek borçsuz daire ve belge oluşturup `validity_status` değerini kontrol eder, borçlu daireyi ayrıca reddettirir. Önceki bellek idempotans önbelleği de işlemle birlikte kaydedilen PostgreSQL yanıtıyla değiştirildi. Önce/sonra davranışı entegrasyon testleriyle kanıtlanır.

### 5. Tahakkuk motorunun doğruluğu

```bash
docker compose --profile verify run --build --rm verify
# veya DATABASE_URL ayarlı backend klasöründe:
go test -v ./tests/... -run 'TestT1|TestIntegration_Accrual|TestIntegration_Ownership'
```

Birim testleri dört dağıtım türünü, muafiyeti ve toplamın tam kuruş eşitliğini denetler. HTTP testi 125.433 kuruşluk tahakkukun yazılmış borçlarını toplayarak doğrular. Malik devri testi 31 günlük ayda 3.100 kuruşu eski malike 1.000, yeni malike 2.100 kuruş olarak kaydeder. Başarı kodu görmek tek başına kanıt sayılmaz.

Test ve teslim sonuçları: [DELIVERY_AUDIT.md](DELIVERY_AUDIT.md).

DDO Case Kodu: KUMSAL-0c7cc1
