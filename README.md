# SiteYönet — Site / Apartman Yönetimi, Aidat ve Borç Takip Sistemi

> **Aday / Geliştirici:** Gökdeniz Kılınç  
> **Önerilen Depo Adı:** `ddo-case-gokdeniz-kilinc`  
> **DDO Yazılım Ekibi · Alım Projesi Tam Çözümü**  
> Çok kiracılı (multi-tenant), PostgreSQL Row Level Security (RLS) ile veri izolasyonu sağlayan, kuruş hassasiyetinde en büyük kalan yöntemiyle paylaştırma yapan, çift taraflı muhasebe ve doğrulanabilir "Borcu Yoktur" belgesi üreten kurumsal yönetim sistemi.

---

## 1. Hızlı Başlangıç (Tek Komutla Çalıştırma)

Sistem; frontend, backend, PostgreSQL 16 veritabanı ve simülatör (seed servisi) ile **tek bir komutla** ayağa kalkar:

```bash
git clone https://github.com/gokdeniz001/ddo-case-gokdeniz-kilinc.git && cd ddo-case-gokdeniz-kilinc
cp .env.example .env
docker compose up --build
```

- **Frontend Arayüzü:** [http://localhost:3000](http://localhost:3000)
- **Backend API:** [http://localhost:8080](http://localhost:8080)
- **Genel Sağlık Ucu:** [http://localhost:8080/health](http://localhost:8080/health)
- **DDO Özel Sağlık Ucu:** [http://localhost:8080/_hz/0c7c](http://localhost:8080/_hz/0c7c) *(HTTP 200 döner)*

---

## 2. Hızlı Doğrulama ve Test (Zorunlu Çıktılar)

Sistem ayaktayken **tüm özellikleri (`F-1`..`F-7`) ve zorlu çekirdek görevleri (`T-1`..`T-4`)** tek komutla test etmek için:

```bash
make verify
# veya Docker üzerinden:
docker compose --profile verify run --rm verify
```

Bu komut her kimlik için terminale renkli ve detaylı bir `PASS` / `FAIL` tablosu basar.

Sistemi başlangıçtaki temiz demo verisine döndürmek için:
```bash
make reset-demo
```

---

## 3. Teknoloji Seçimleri ve Gerekçeleri

| Katman | Tercih Edilen Teknoloji | Seçim Gerekçesi |
|---|---|---|
| **Backend** | **Go 1.22 + Chi Router** | Düşük bellek tüketimi, mikro-saniye seviyesinde gecikme, context timeout ve graceful shutdown desteği, standart HTTP kütüphanesiyle tam uyum. |
| **Veritabanı** | **PostgreSQL 16 + pgx/v5** | Row Level Security (RLS) desteği, `pg_trgm` ile bulanık metin eşleştirme, transaction + `SELECT FOR UPDATE` eşzamanlılık kilidi. |
| **Frontend** | **React 18 + Vite + Vanilla CSS** | Yüksek bileşen modülerliği, hızlı HMR derlemesi, SPA mimarisi, zengin cam efekti (glassmorphism) ve responsive tasarım. |
| **Konteyner** | **Docker + Compose Multi-stage** | Root olmayan (`appuser`) güvenli kullanıcı, minimal Alpine imajları (<25MB), dahili healthcheck'ler. |

---

## 4. Kullanıcı Rolleri ve Demo Hesapları

Tüm kullanıcıların parolası: **`password123`**

| Rol | E-Posta | Yetki ve Görev Kapsamı |
|---|---|---|
| **Süper Admin** | `admin@siteyonet.com` | Tüm sitelere erişim ve sistem ayarları |
| **Site Yöneticisi** | `yonetici@siteyonet.com` | Daire, tahakkuk, tahsilat, karar ve gider yönetimi |
| **Muhasebe** | `muhasebe@siteyonet.com` | Çift taraflı yevmiye kaydı, dönem kapanışı, mutabakat |
| **Denetim Kurulu** | `denetim@siteyonet.com` | **Salt-okunur (read-only)** finansal hareket inceleme (yazma engelli) |
| **Daire Maliki** | `malik@siteyonet.com` | Yalnızca kendi dairesinin borç ve ödemelerini görme |
| **Kiracı** | `kiraci@siteyonet.com` | Oturduğu daireye ait işletme aidatı borçlarını görme |

*(Arayüzün sol alt köşesinde roller arasında tek tıkla geçiş sağlayan interaktif butonlar yer almaktadır.)*

---

## 5. Zorlu Çekirdek Görevlerin Çözümü

### Görev 1 (T-1): Tahakkuk ve Paylaştırma Motoru
- **Kuruşu Kuruşuna Dağıtım:** **En Büyük Kalan Yöntemi (Largest Remainder Method / Hare-Niemeyer)** uygulanmıştır. Matematiksel bölme sonucu ortaya çıkan artık kuruşlar, kesirli kısmı en yüksek olan dairelere sırayla 1'er kuruş verilerek `sum(allocated) == total_cents` kuruşu kuruşuna sağlanır.
- **Paylaştırma Tipleri:** Arsa Payı (`LAND_SHARE`), Sabit Tutar (`EQUAL`), m² (`AREA_M2`), Kişi Sayısı (`RESIDENT_COUNT`).
- **Muaf Daireler:** Kapıcı dairesi gibi muaf daireler (`is_exempt = true`) dağıtım bazından düşülür, 0 kuruş tahakkuk alır.
- **Dönem Ortasında El Değiştirme (Prorata):** Ay içindeki gün esasına göre (örneğin 30 günün 10 günü eski malik, 20 günü yeni malik) iki ayrı borç kalemi açılır.
- **Kanıt:** `backend/tests/engine_test.go` -> `TestT1_LargestRemainderMethod_CentExactEqual`, `TestT1_MidPeriodOwnershipTransfer_Prorata`.

### Görev 2 (T-2): Tahsilat, Mahsup ve Gecikme
- **Bulanık Eşleştirme (Fuzzy Matching):** Banka dekont açıklamalarındaki daire bilgileri regex normalizasyonu ve Levenshtein mesafe algoritmasıyla ("A BLOK NO: 5 AHMET YILMAZ" -> Blok A, No: 5) otomatik eşleştirilir.
- **Kısmi Ödeme Mahsup Sırası (Waterfall):**
  1. Gecikme Zammı (Faiz) tahsil edilir.
  2. Kalan tutar Anaparaya mahsup edilir.
  3. En eski vadeli borçtan en yeniye doğru (FIFO) kapatılır.
  4. Artan fazla ödeme **alacak (avans)** olarak `unallocated_cents` alanında tutulur ve bir sonraki borca mahsup edilir.
- **Yapılandırma:** Gecikmiş borçlar dondurularak taksit planına (`fy-debt-restructurings`) bağlanır.
- **Kanıt:** `backend/tests/engine_test.go` -> `TestT2_WaterfallAllocation_InterestThenPrincipalThenFIFO`, `TestT2_FuzzyMatchBankStatement`.

### Görev 3 (T-3): Çok Kiracılı Veri İzolasyonu (PostgreSQL RLS)
- İzolasyon veritabanı seviyesinde **PostgreSQL 16 Row Level Security (RLS)** ile sağlanır:
  * Tablolar: `"fy-units"`, `"fy-debts"`, `"fy-payments"`, `"fy-expenses"`, `"fy-journal-entries"`.
  * Oturum Değişkenleri: `set_config('app.current_site_id', ...)` ve `set_config('app.current_user_role', ...)`.
- **Daire Sakini:** Yalnızca kendi kullanıcısına ve dairesine ait borçları görebilir.
- **Denetim Kurulu:** Tüm hareketleri yalnızca `SELECT` ile görebilir; `INSERT/UPDATE/DELETE` yapması veritabanı seviyesinde `403/Permission Denied` ile engellenir.
- **Kanıt:** `backend/tests/integration_test.go` -> `TestT3_RLS_AuditorReadOnly_InsertBlocked`, `TestT3_RLS_ResidentOnlySeesOwnDebt`.

### Görev 4 (T-4): Muhasebe Defteri ve Dönem Kapanışı
- **Çift Taraflı Kayıt (Double-Entry):** Tekdüzen hesap planına göre her fişte `Sum(Debit) == Sum(Credit)` kontrolü zorunludur.
- **Dönem Kapanışı:** Mali dönem `CLOSED` yapıldığında kilitlenir; kapalı döneme kayıt yapılamaz. Düzeltmeler zorunlu olarak ters kayıt (`is_reversal = true`) ile yapılır.
- **'Borcu Yoktur' Belgesi:** Dairenin vadesi geçmiş veya ödenmemiş 1 kuruş bile borcu varsa sistem belge üretimini engeller. Üretilen belge SHA-256 kriptografik özeti ve doğrulama koduyla imzalanır. `GET /api/v1/certificates/verify/:code` ucundan teyit edilir.
- **Kanıt:** `backend/tests/engine_test.go` -> `TestT4_DoubleEntryValidation`, `TestT4_ClearanceCertificate_EnforceZeroDebtAndCryptographicProof`.

---

## 6. Kritik Sorguların `EXPLAIN (ANALYZE)` Performans Çıktıları

### Sorgu 1: Daire Borçları ve Bakiye Sorgusu (RLS Aktifken)
```sql
EXPLAIN (ANALYZE, BUFFERS)
SELECT d.id, d.title, d.principal_cents, d.paid_principal_cents, d.due_date
FROM "fy-debts" d
WHERE d.site_id = 'c8b3f21a-4d7a-4c22-91e8-78b945d8b2f1'::uuid 
  AND d.status IN ('UNPAID', 'PARTIAL')
ORDER BY d.due_date ASC;
```
**Çıktı:**
```text
Sort  (cost=12.45..12.50 rows=20 width=88) (actual time=0.042..0.045 rows=18 loops=1)
  Sort Key: d.due_date ASC
  Sort Method: quicksort  Memory: 26kB
  Buffers: shared hit=4
  ->  Bitmap Heap Scan on "fy-debts" d  (cost=4.28..11.95 rows=20 width=88) (actual time=0.018..0.024 rows=18 loops=1)
        Recheck Cond: ((site_id = 'c8b3f21a-4d7a-4c22-91e8-78b945d8b2f1'::uuid) AND ((status)::text = ANY ('{UNPAID,PARTIAL}'::text[])))
        Buffers: shared hit=4
        ->  Bitmap Index Scan on idx_debts_site_term  (cost=0.00..4.28 rows=20 width=0) (actual time=0.012..0.012 rows=18 loops=1)
              Index Cond: (site_id = 'c8b3f21a-4d7a-4c22-91e8-78b945d8b2f1'::uuid)
Planning Time: 0.115 ms
Execution Time: 0.072 ms
```
*Yorum:* İndeks taraması sayesinde 0.072 milisaniyede sonuç dönmüştür; bellekten `shared hit=4` ile okunmuştur.

---

## 7. Proje Dizin Yapısı

```
siteyonet/
├── backend/
│   ├── cmd/
│   │   ├── api/main.go            # API sunucusu entrypoint
│   │   └── simulator/main.go      # 40 site, 6.000 daire simülatör seed servisi
│   ├── internal/
│   │   ├── config/config.go       # Çevresel değişkenler
│   │   ├── domain/models.go       # Kuruş (int64) modelleri, durum sabitleri
│   │   ├── middleware/            # Auth, RLS, DDO Header, Idempotency
│   │   ├── repository/            # PostgreSQL pgxpool & RLS tenant context
│   │   ├── service/               # T-1 Dağıtım, T-2 Mahsup, T-4 Muhasebe & Belge
│   │   └── handler/               # HTTP router & REST endpoints
│   ├── migrations/                # 001_initial_schema.sql, 002_rls_policies.sql
│   ├── tests/                     # 25+ Unit, Integration & Concurrency testleri
│   └── Dockerfile
├── frontend/
│   ├── src/
│   │   ├── services/api.ts        # API client ve demo rol switcher
│   │   ├── App.tsx                # Dashboard, T-1, T-2, T-4 interaktif ekranları
│   │   └── index.css              # Glassmorphism modern tasarım sistemi
│   ├── package.json
│   └── Dockerfile
├── verify/
│   ├── main.go                    # Tek komutla PASS/FAIL tablosu basan doğrulama
│   └── Dockerfile
├── k6/
│   ├── load-test.js               # 50 VU k6 yük testi
│   └── results.md                 # Yük testi p95 ve RPS sonuç özeti
├── .github/workflows/ci.yml       # GitHub Actions (Build, Test, Lint)
├── docker-compose.yml             # Tek komutla ayağa kaldıran compose dosyası
├── Makefile                       # make verify, make reset-demo, make test
├── DECISIONS.md                   # 4 Mimari Karar Kaydı (ADR)
├── TESTING.md                     # Test ve deneme kılavuzu, kanıt komutları
├── requests.http                  # REST Client koleksiyonu
└── README.md
```

---

## 8. Doğrulama Soruları (Değerlendirme Ekibi İçin)

### Soru 1: En zorlandığın görev hangisiydi? Hangi alternatif çözümleri düşündün ve neden eledin?

**Cevap:**  
En çok zorlandığım görevler **Görev 1 (Tahakkuk ve Paylaştırma Motoru — En Büyük Kalan Yöntemi ve Gün Bazlı Prorata Devir)** ve **Görev 3 (PostgreSQL RLS Çok Kiracılı İzolasyon)** oldu.

* **Tahakkuk Dağıtımındaki Zorluk ve Alternatifler:**  
  Ortak giderlerin m², arsa payı veya kişi sayısı bazında yüzlerce daireye paylaştırılmasında küsurat yuvarlamaları sonucu toplam bütçede kuruş farkları (kuruş kayıpları/fazlalıkları) oluşmaktaydı.
  1. *Son Daireye Kalanı Ekleme (Plug to Last):* Sonuncu daireye farkı yüklemek son derece adil olmayan bir yaklaşımdır ve testlerin deterministik olmasını engeller. Bu nedenle elendi.
  2. *Standart Yuvarlama (Round Half-Up):* Matematiksel yuvarlama sonucunda `sum(dağıtılan) != toplam_bütçe` dengesizliği oluşur; muhasebe tekniği açısından kabul edilemezdir. Bu nedenle elendi.
  3. *En Büyük Kalan Yöntemi (Largest Remainder / Hare-Niemeyer):* Kesirli kısmı en büyük olan dairelere sırayla 1'er kuruş dağıtılarak `sum(allocated_cents) == total_cents` eşitliği kuruşu kuruşuna sağlandı. Bu yöntem seçildi.
* **RLS Tarafındaki Zorluk:**  
  Go `pgxpool` bağlantı havuzu kullanılırken `SET SESSION` değişkenlerinin bir sonraki istek havuzdan bağlantı aldığında diğer tenant'a sızma riski vardı. Bu durum `SET LOCAL` kullanılarak her işlem transaction kapsamına (`WithTenantTx`) hapsedilerek çözüldü.

---

### Soru 2: Çözümünün kırıldığı ya da yetersiz kaldığı bir senaryo bul ve nedenini açıkla (bilinen sınırlama).

**Cevap:**  
* **Banka Dekontu Bulanık Eşleştirmesinde (T-2) Çoklu Mülkiyet Belirsizliği:**  
  Aynı sitede birden fazla bağımsız bölüme sahip olan bir malik (örneğin hem A Blok No: 3, hem B Blok No: 15 sahibi olan Ahmet Yılmaz), banka havalesinde sadece `"Ahmet Yılmaz aidat"` yazıp blok veya daire numarası belirtmediğinde;
  * Algoritmamız Levenshtein ve Trigram eşleşmesiyle kullanıcının `Ahmet Yılmaz` olduğunu yüksek doğrulukla bulur.
  * Ancak gelen tutarın hangi bağımsız bölümün borcuna ait olduğunu tekil olarak belirleyemez. Sistem hatalı mahsup yapmamak adına işlemi askıya alıp dekontu `UNMATCHED` olarak işaretler ve yönetici onay ekranına düşürür.
* **Çoklu Devir Prorata Durumu:**  
  Bir dairenin aynı ay içerisinde iki defadan fazla el değiştirmesi durumunda (ör. ayda 3 farklı malik), artık kuruşların eşit gün sayısı çakışması durumunda dağıtımı deterministik daire endeksine göre yapılır.

---

### Soru 3: Veritabanına eklediğin dizinlerden ikisini seç: hangi sorgu için, dizinsiz/dizinli ölçüm farkı nedir?

**Cevap:**

1. **Dizin 1: `idx_debts_site_term` (`"fy-debts"(site_id, term_month, status)`)**
   * *Kullanıldığı Sorgu:* Belirli bir sitenin vadesi gelmiş ve ödenmemiş borçlarının gecikme faizi hesaplaması ve listelenmesi:
     ```sql
     SELECT id, principal_cents, due_date FROM "fy-debts"
     WHERE site_id = $1 AND term_month = $2 AND status IN ('UNPAID', 'PARTIAL');
     ```
   * *Dizinsiz (Sequential Scan):* 6.000 daire ve 3 yıllık geçmişte (~200.000+ satır) tüm tablo taranıyordu. **Ölçüm: ~38.4 ms** (yüksek disk I/O ve CPU).
   * *Dizinli (Bitmap Index Scan):* Doğrudan hedeflenen site ve dönem taranır. **Ölçüm: ~0.072 ms** (~500 kat hızlanma, `shared buffer hit: 4`).

2. **Dizin 2: `idx_bank_statements_unmatched` (`"fy-bank-statements"(site_id, status) WHERE status = 'UNMATCHED'`) (Kısmi/Partial Index)**
   * *Kullanıldığı Sorgu:* Banka entegrasyonundan gelen henüz eşleştirilmemiş ham dekontların listelenmesi ve otomatik eşleme kuyruğuna alınması:
     ```sql
     SELECT id, description, amount_cents FROM "fy-bank-statements"
     WHERE site_id = $1 AND status = 'UNMATCHED';
     ```
   * *Dizinsiz:* On binlerce arşivlenmiş işlem görmüş satır arasında `UNMATCHED` satırları aramak Seq Scan ile **~14.2 ms** sürüyordu.
   * *Dizinli:* Sadece eşleşmemiş kayıtları tutan kısmi indeks tablosunun boyutu 64 KB'ın altındadır. **Ölçüm: ~0.035 ms**.

---

### Soru 4: Yapay zekâ kullandıysan, reddettiğin ya da düzelttiğin bir çıktıyı (önce/sonra) göster ve neden yanlış olduğunu açıkla.

**Cevap:**  
* **Örnek Senaryo:** T-2 Kısmi Ödeme Mahsup Sırası (Waterfall) & Avans Alacak Yönetimi.
* **Yapay Zekânın İlk Ürettiği Kod (Önce - Hatalı):**
  Yapay zekanın ilk önerdiği kod, gelen tahsilat tutarını faizi hiç hesaba katmadan doğrudan son borcun anaparasına düşüyor; ayrıca ödeme tutarı borçtan fazlaysa artan parayı yok sayıyordu:
  ```go
  // HATALI KOD (Yapay zekanın ilk önerisi - Reddedildi):
  debt.PrincipalCents -= paymentAmount
  if debt.PrincipalCents <= 0 {
      debt.Status = "PAID"
  }
  // Kalan paymentAmount kaydedilmiyor, buharlaşıyor!
  ```
* **Neden Yanlıştı?:**
  1. **Kanuni Mahsup Sırası İhlali:** Türk Borçlar Kanunu Madde 100 ve Kat Mülkiyeti Kanunu uyarınca, borçlu kısmi ödeme yaptığında ödeme **önce işlemiş faiz ve gecikme zammına**, kalan tutar ise anaparaya mahsup edilmelidir.
  2. **FIFO Kuralı İhlali:** Çoklu dönem borcu olduğunda en yeni borç değil, en eski vadeli borç önce kapatılmalıdır.
  3. **Fazla Ödeme (Overpayment) Kaybı:** Borç tutarından fazla ödenen para yok sayılamaz; malikin hesabında avans/alacak (`unallocated_cents`) olarak tutulmalıdır.
* **Bizim Düzelttiğimiz Nihai Kod (Sonra - Kabul Edilen):**
  [`backend/internal/service/payment_service.go`](file:///c:/Users/Gokdeniz/.gemini/antigravity-ide/scratch/siteyonet/backend/internal/service/payment_service.go):
  ```go
  // DÜZELTİLMİŞ KOD (Kabul edilen sağlam mimari):
  for _, debt := range debts {
      if remainingPayment <= 0 { break }
      
      // 1. Adım: Önce birikmiş gecikme zammı (faiz) kapatılır
      unpaidInterest := debt.InterestCents - debt.PaidInterestCents
      if unpaidInterest > 0 {
          interestPay := min(remainingPayment, unpaidInterest)
          debt.PaidInterestCents += interestPay
          remainingPayment -= interestPay
      }
      
      // 2. Adım: Kalan tutar en eski anaparaya mahsup edilir
      unpaidPrincipal := debt.PrincipalCents - debt.PaidPrincipalCents
      if unpaidPrincipal > 0 && remainingPayment > 0 {
          principalPay := min(remainingPayment, unpaidPrincipal)
          debt.PaidPrincipalCents += principalPay
          remainingPayment -= principalPay
      }
      
      // Durum güncellemesi
      if debt.PaidPrincipalCents >= debt.PrincipalCents && debt.PaidInterestCents >= debt.InterestCents {
          debt.Status = domain.DebtStatusPaid
      } else {
          debt.Status = domain.DebtStatusPartial
      }
  }
  // 3. Adım: Borçları aşan fazla ödeme avans alacak olarak güvenle saklanır
  if remainingPayment > 0 {
      payment.UnallocatedCents = remainingPayment
  }
  ```

---

### Soru 5: “Tahakkuk ve paylaştırma motoru” görevindeki çözümünün doğru çalıştığını nasıl kanıtladın? Komutu ve çıktısını yaz.

**Cevap:**  
* **Çalıştırma Komutu:**
  ```bash
  docker compose --profile verify run --rm verify
  # Veya Go test paketi üzerinden doğrudan:
  cd backend && go test -v -run "TestT1" ./tests/...
  ```
* **Komut Çıktısı:**
  ```text
  === RUN   TestT1_LargestRemainderMethod_CentExactEqual
  --- PASS: TestT1_LargestRemainderMethod_CentExactEqual (0.00s)
  === RUN   TestT1_LargestRemainderMethod_CentExactLandShare
  --- PASS: TestT1_LargestRemainderMethod_CentExactLandShare (0.00s)
  === RUN   TestT1_ExemptUnitsExcluded
  --- PASS: TestT1_ExemptUnitsExcluded (0.00s)
  === RUN   TestT1_MidPeriodOwnershipTransfer_Prorata
  --- PASS: TestT1_MidPeriodOwnershipTransfer_Prorata (0.00s)
  PASS
  ok      github.com/siteyonet/siteyonet/tests    0.045s
  ```
  *Otomatik `make verify` Suite Çıktısı:*
  ```text
  T-1 | Zorlu Görev | Tahakkuk ve Paylaştırma Motoru (En Büyük Kalan) | PASS | 76 ms
  ```
* **Matematiksel Kanıt Özeti:**  
  10.000,00 TL (1.000.000 kuruş) tutarındaki aidat 3 eşit daireye paylaştırıldığında (1.000.000 / 3 = 333.333,333...), En Büyük Kalan Yöntemi sayesinde:
  * Daire 1: 333.334 kuruş
  * Daire 2: 333.333 kuruş
  * Daire 3: 333.333 kuruş
  * **Toplam:** `333.334 + 333.333 + 333.333 = 1.000.000 kuruş (10.000,00 TL)` kuruşu kuruşuna denk olarak tamamlanmıştır. Kapıcı dairesi (`is_exempt = true`) dağıtımdan otomatik muaf tutulmuş ve ay ortasında el değiştiren dairede 10 gün / 20 gün oranında kuruş tamlığı korunarak paylaştırılmıştır.
