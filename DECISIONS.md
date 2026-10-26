# Mimari Karar Kayıtları (Architecture Decision Records — ADR)

Bu belge, **SiteYönet** mimarisinin tasarımında verilen temel teknik ve stratejik kararları, değerlendirilen alternatifleri ve sonuçlarını belgeler.

---

## ADR-001: Para Tutarlarının Temsili ve Kuruş Hassasiyeti

### Bağlam
Site ve apartman yönetim sistemlerinde aidat tahakkukları, gecikme zamları ve gider paylaşımları sürekli matematiksel bölme işlemlerine tabidir. Kayan noktalı sayılar (float32, float64), IEEE 754 standardı gereği ikili tabanda hassasiyet kaybına (örneğin `0.1 + 0.2 = 0.30000000000000004`) yol açar. Bu kayıp, yüzlerce dairede kuruş farklarına ve muhasebe dengesinin bozulmasına neden olur.

### Seçenekler
1. **IEEE 754 Float (float64):** Hızlıdır ancak yuvarlama hataları birikir, yasal muhasebede kabul edilemez.
2. **PostgreSQL NUMERIC / Arbitrary Precision Decimal:** Hassastır ancak CPU maliyeti ve ORM/sürücü uyumsuzlukları doğurabilir.
3. **Tamsayı Kuruş (BIGINT / int64):** Tüm parasal değerler 1 TL = 100 kuruş olarak saklanır (örneğin 1.500,00 TL = 150.000).

### Karar
Tüm veritabanı şemasında ve backend domain modellerinde para tutarları kesin olarak **int64 (BIGINT)** kuruş olarak tutulacaktır. Arayüz ve raporlama katmanında sadece gösterim esnasında 100'e bölünerek biçimlendirilir.

### Sonuçlar
- **Artılar:** Sıfır yuvarlama hatası; CPU üzerinde en hızlı tamsayı aritmetiği; int64 üst sınırı 9.223.372.036.854.775.807 kuruştur; bu sınır aritmetik taşmayı kendiliğinden engellemez.
- **Eksiler:** İstemci ve sunucu arasında veri taşınırken formatlama fonksiyonlarına ihtiyaç duyulur.

---

## ADR-002: Çok Kiracılı Veri İzolasyonunda PostgreSQL Row Level Security (RLS)

### Bağlam
Sistem aynı anda 40+ farklı siteyi (tenant) yönetmektedir. Daire maliki ve kiracı yalnızca kendi dairesine ait borçları görmeli, site yöneticisi sadece kendi sitesini yönetmeli, denetim kurulu ise yalnızca kendi sitesini salt-okunur görebilmelidir. Kod katmanında `WHERE site_id = ...` yazmayı unutmak ciddi bir veri sızıntısı (data leak) riski oluşturur.

### Seçenekler
1. **Veritabanı Seviyesinde İzolasyon (Site başına ayrı veritabanı / şema):** Yüksek izolasyon ancak 40 site için bağlantı havuzu ve migrasyon yönetimi çok ağırdır.
2. **Uygulama Seviyesi İzolasyon (Application-layer WHERE filtresi):** İnsan hatasına açıktır; tek bir sorguda `WHERE site_id` unutulursa veri sızar.
3. **PostgreSQL Row Level Security (RLS):** Veritabanı motorunun kendisi her SQL sorgusuna otomatik satır bazlı filtre uygular.

### Karar
PostgreSQL 16'nın yerleşik **Row Level Security (RLS)** mekanizması seçilmiştir. Backend HTTP middleware'i, her doğrulanmış istekte `SET LOCAL app.current_site_id = ...`, `app.current_user_id = ...` ve `app.current_user_role = ...` değişkenlerini belirler.

### Sonuçlar
- **Artılar:** Güvenlik veritabanı çekirdeğinde garanti edilir; yazılımcı sorguda `WHERE` yazmayı unatsa bile veritabanı satırı döndürmez; denetçinin `INSERT/UPDATE` yetkisi doğrudan SQL seviyesinde engellenir.
- **Eksiler:** Veritabanı bağlantı havuzunda oturum değişkenlerinin her transaction başında temizlenmesi/ayarlanması gerekir (`repository.WithTenantTx`).

---

## ADR-003: Aidat Dağıtımında En Büyük Kalan Yöntemi (Largest Remainder Method)

### Bağlam
Toplam 10.000,00 TL'lik bir ortak gider 3 daireye eşit dağıtıldığında daire başına `3.333,333...` TL düşer. İki basamağa yuvarlandığında `3.333,33 * 3 = 9.999,99 TL` eder ve 1 kuruş kaybolur. DDO yönergesi dağıtım toplamının **kuruşu kuruşuna** tutmasını şart koşmaktadır.

### Seçenekler
1. **Rastgele Daireye Ekleme:** Sonuncu daireye farkı eklemek adil değildir ve testlerde tutarsızlık yaratır.
2. **Standart Yuvarlama (Round half-up):** Toplam bütçe ile dağıtılan toplam arasında artı veya eksi farklar oluşturur.
3. **En Büyük Kalan Yöntemi (Hare-Niemeyer / Largest Remainder):** Kesirli kısmı en büyük olan dairelere sırayla 1'er kuruş verilerek toplamın tam denk olması sağlanır.

### Karar
Tahakkuk motorunda (Task 1) **En Büyük Kalan Yöntemi** uygulanmıştır. Her dairenin teorik payı hesaplanır, taban kuruş atanır; kalan kuruşlar ise küsuratı en büyük olan dairelere sıralı dağıtılır.

### Sonuçlar
- **Artılar:** `sum(allocated_cents) == total_cents` eşitliği %100 matematiksel olarak garanti edilir. Kalan kuruşların dağıtım kuralı açık ve tekrarlanabilirdir.
- **Eksiler:** Dağıtım öncesinde sıralama (sort) maliyeti vardır (küçük listelerde ihmal edilebilir).

---

## ADR-004: Çift Taraflı Muhasebe (Double-Entry) ve Dönem Kilitleme

### Bağlam
Geleneksel apartman programları sadece tek taraflı gelir-gider listesi tutar. Bu durum kasa-banka mutabakatı yapılmasını ve bütçe sapmalarının denetlenmesini imkansız kılar. Ayrıca geriye dönük silme/değiştirme işlemleri suiistimale yol açar.

### Seçenekler
1. **Tek Taraflı Defter (Single-entry cash log):** Basittir ancak kasa/banka açıkları tespit edilemez.
2. **Çift Taraflı Yevmiye Defteri (Double-entry):** Her işlemde Borç (Debit) = Alacak (Credit) eşitliği aranır. Kapanan dönem kilitlenir; düzeltmeler ters kayıtla yapılır.

### Karar
Tekdüzen Hesap Planına uygun Çift Taraflı Muhasebe Defteri (`fy-journal-entries`, `fy-journal-lines`, `fy-ledger-accounts`) seçilmiştir. Dönem kapandığında `status = 'CLOSED'` yapılır ve veritabanı/uygulama seviyesinde yeni kayıt engellenir. Düzeltmeler zorunlu olarak ters kayıt fişi (Reversal Entry) açılarak yapılır.

### Sonuçlar
- **Artılar:** Dengeli yevmiye kayıtları ve dönem kilidi; mutabakat içe aktarılmış ekstrelerle sınırlıdır.


## ADR-005: İstek boyunca PostgreSQL işlemi ve RLS rolü

Bağlam: İlk sürümde havuz sorguları PostgreSQL süper kullanıcısıyla çalışıyor, RLS dosyaları gerçek API sorgularını korumuyordu.
Seçenekler: Yalnızca WHERE site_id filtreleri, her sorguya ayrı işlem, istek boyunca tek işlem.
Karar: Korunan istekte SET LOCAL ROLE siteyonet_runtime; site/kullanıcı/rol ayarları transaction-local. Tüm sorgular aynı işlemden geçer. HTTP yanıtı commit sonrasına kadar tamponlanır.
Sonuç: RLS, havuza geri dönen bağlantılarda bağlam taşımadan çalışır. İşlem başarısızsa başarı yanıtı gönderilmez. Uzun yanıtlar bellekte tutulur; akış API'si amaçlanmamıştır.

## ADR-006: Site yazma kilidi ve kalıcı idempotans

Bağlam: Borç okuma ile ödeme yazma arasındaki yarış kayıp güncelleme oluşturabiliyordu; bellek önbelleği yeniden başlatmada kayboluyordu.
Seçenekler: Satır kilitleri, serializable/retry, site bazlı transaction advisory lock.
Karar: Bu demo için site bazlı yazma kilidi; iş etkisi ile idempotans yanıtı aynı PostgreSQL işleminde. Anahtar farklı istek gövdesi/yol/kullanıcı ile tekrar kullanılırsa 409.
Sonuç: Aynı siteye yazmalar sıralanır, farklı siteler bağımsızdır. Yazma throughput'u bilinçli olarak sınırlanır. Sekiz paralel farklı ödeme ve sekiz aynı anahtarlı ödeme gerçek PostgreSQL testleriyle doğrulanır.

## ADR-007: Kuruş hesabında rasyonel sayılar ve liste sayfalama

Bağlam: Para dağıtımındaki kayan nokta ara hesapları ve sınırsız borç listeleri doğruluk/performans riski oluşturuyordu.
Seçenekler: float64, sabit ölçekli tamsayılar, math/big.Rat; offset veya imleç sayfalama.
Karar: Dağıtım ve gecikme tutarları rasyonel aritmetik ile kuruşa yuvarlanır; borç listesi (due_date,id) imleci ve bileşik dizin kullanır.
Sonuç: En büyük kalan toplamı tam tutar. Varsayılan 500, üst sınır 1.000 kayıt; frontend sayfaları birleştirir. Yük testi sonuçları audit/ altında saklanır.
