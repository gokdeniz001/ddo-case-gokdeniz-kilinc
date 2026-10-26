# Teslim Öncesi Denetim — 26 Ekim 2026

## Sonuç ve teslim durumu

Uygulama yerel Docker ortamında canlı denendi. Önemli güvenlik, test, tahsilat ve performans hataları düzeltildi. Aşağıdaki kanıtlar çalışan kapsamı gösterir; bilinen sınırlar nedeniyle bu rapor tüm olası muhasebe senaryolarının tamamlandığı iddiası değildir.

Değişiklikler yereldedir; bu çalışma GitHub'a push veya teslim anketi gönderimi yapmadı. Son tarih **26 Ekim 2026 23.59 (Türkiye saati)**. Yeni commitlerin GitHub Actions sonucu, push yapılmadan doğrulanamaz.

## Ekran görüntüsündeki hata

`39bb29f` GitHub'a başarıyla yüklenmişti. Kırmızı işaret yükleme hatası değil, `Backend Build, Test & Lint` işinin `Run golangci-lint` adımındaki başarısızlığıydı. Frontend işi geçmişti. İnceleme başındaki son `8a05795` çalışması başarılıydı.

- [Görüntüdeki başarısız çalışma](https://github.com/gokdeniz001/ddo-case-gokdeniz-kilinc/actions/runs/40365720484)
- [İnceleme başındaki başarılı çalışma](https://github.com/gokdeniz001/ddo-case-gokdeniz-kilinc/actions/runs/40368658213)

## Bulunan ve düzeltilen başlıca sorunlar

| Bulgu | Düzeltme |
|---|---|
| RLS dosyaları olmasına rağmen API süper kullanıcı havuzuyla doğrudan sorguluyordu | Korunan her istekte kısıtlı runtime rolü ve transaction-local tenant bağlamı |
| Bazı anonim istekler veri döndürüyor veya nil kullanıcı nedeniyle hata veriyordu | Tüm korunan yolların başında 401/rol denetimi |
| RLS testleri boş verilere bakıyor, SQL hatalarını yok sayıyor veya SKIP ediyordu | Her test bağımsız dolu fixture kuruyor; veritabanı yoksa FAIL |
| Gerçek veride üç eski entegrasyon testi şemaya uymadığı için başarısızdı | Güncel şema üzerinde HTTP ve doğrudan SQL sonuç testleri |
| T-4 kabul testi hata halinde bile başarılı sayılıyordu | Gerçek belge üretimi ve imza/debt kontrolü; hata artık FAIL |
| Ödeme okuma/yazma yarışı ve boş idempotans anahtarı UNIQUE hatası | Site yazma kilidi, aynı transaction içinde idempotans, boş anahtarda NULL |
| Bellek yanıt önbelleği restart sonrası kayboluyordu | Kalıcı veritabanı yanıtı; farklı gövde/yol/kullanıcı için 409 |
| Audit kaydı goroutine içinde kaybolabiliyordu | İş etkisi ve audit aynı transaction; runtime rolü UPDATE/DELETE yapamaz |
| `/auth/me` 404 ve dekont JSON alanları arayüzle uyumsuzdu | Route çakışması giderildi; JSON alan adları düzeltildi |
| NULL eşleşme puanı seed ekstrelerinin sessizce listeden atılmasına yol açıyordu | COALESCE ve NULL satırı içeren regresyon kontrolü |
| Varsayılan seed 5 site, 100 daire, üç aydı | Temiz varsayılan 40 site, 6.000 daire, 36 ay; COPY ile toplu geçmiş üretimi |
| Demo malik/kiracı hesapları gerçek daireye bağlı değildi | Temiz seed'de bir daireye gerçek bağlantı |
| Tahsilat ekranı yalnızca dekont eşleştiriyor, ödeme kaydetmiyordu | Gerçek ödeme formu ve CSV içe aktarma |
| Duyuru/karar/bakım ekranı eksikti | Üç sekmeli gerçek veri/form ekranı, bakım durum geçişi |
| Malik devri yalnızca birim motorunda vardı | API devri, occupancy geçmişi, API tahakkukta gün esaslı pay |
| Aynı numara/kattaki farklı daire ve blok yanlış eşleşebiliyordu | Kat numarası kaldırıldı; açık blok/daire uyuşmazlığı elenir, belirsiz yakın skorlar reddedilir |
| Tahakkuk/ücret ara hesaplarında float para hesabı vardı | math/big.Rat ile kuruş hesaplama |
| Ödeme avansı/faizi bütünüyle aidat alacağı hesabına yazılıyordu | Anapara, gecikme geliri ve avans ayrı hesaplara |
| Ödenmemiş fatura banka çıkışı sayılıyordu | Tedarikçi borcu hesabı; nakit akışında yalnızca ödenen gider |
| Tahakkuk geri alma muhasebe ters fişini oluşturmuyordu | Açık/ödenmemiş tahakkukta bağlı fişin gerçek ters kaydı |
| Bütçe hedefleri API kodunda sabitti | RLS korumalı bütçe tablosu, yetkili kayıt ucu, gerçek karşılaştırma |
| Sınırsız borç listesi 50 kullanıcılı yükte p95 hedefine uymuyordu | (due_date,id) keyset, 500 varsayılan kayıt ve bileşik dizin |
| Refresh ve çoklu site geçişi yoktu | Dönen tek kullanımlık refresh token ve üyelik doğrulamalı site geçişi |
| Migrasyon hataları yutuluyor, her başlangıçta tekrar çalıştırılıyordu | Sürümlü, transaction içinde migrasyon; hata halinde başlatma durur |
| Docker build go.sum doğrulamasını kapatıyordu | Kilit dosyaları kullanılır; go mod verify ve npm ci |
| Frontend root çalışıyor, build TypeScript kontrol etmiyordu | nginx kullanıcısı ve tsc --noEmit |
| README'deki performans sayıları kanıt dosyasına bağlı değildi | Gerçek k6 ve EXPLAIN çıktılarıyla değiştirildi |

## Doğrulama kanıtları

- **38 Go testi**, bunların **22'si gerçek PostgreSQL entegrasyon testi**; `-race` ve sabit `golangci-lint v1.64.8` ile başarılı. [Çıktı](audit/final-go-tests.txt).
- Tek komut doğrulaması bütün Go testlerini, ardından 19 satırlık sağlık/auth/F/T API tablosunu çalıştırır. [Son API çıktısı](audit/final-verify.txt).
- Tam seed: **40 site, 6.000 daire, 214.560 borç, 36 ay**. Son ölçüm yaklaşık 78 saniye. [Seed günlüğü](audit/full-seed.txt). Süre imaj indirme/derleme hariçtir; boş Docker önbelleğiyle beş dakika koşulu bu makinede ayrıca ölçülmedi.
- Simülatörün ürettiği CSV'ler: [örnek STE01](audit/generated-bank-statements/STE01.csv).
- 60 saniye / en fazla 50 kullanıcı k6 testi: önce p95 668,12 ms (FAIL), sayfalama sonrası 32,12 ms (PASS). HTTP hata oranı %0, kontroller %100. [Önce](audit/load-before-pagination.json), [sonra](audit/load-after-pagination.json).
- İki indeks için gerçek RLS rolünde plan: [betik](audit/explain.sql), [çıktı](audit/explain.txt). Karşılaştırma sıralı taramayı planner ayarıyla zorlar; dizinlerin fiziksel silinmesi veya kontrollü soğuk önbellek deneyi değildir.
- Arayüzde gerçek duyuru ve **1 TL demo tahsilatı** oluşturuldu. [Duyuru ekranı](audit/community-live.png), [tahsilat ekranı](audit/payment-live.png).
- Frontend TypeScript + Vite build başarılı. Desktop canlı deneme yapıldı; dar ekran CSS'i düzenlendi fakat fiziksel mobil cihaz testi yapılmadı.

## Sınırlar ve görüşmede açıkça söylenmesi gerekenler

1. Aynı ayda birden fazla malik devri desteklenmez; hata döner. Tek devir, 31 günlük ayda 10/21 gün hesabıyla API üzerinden testlidir.
2. Ödeme almış veya kapalı dönemdeki tahakkuk için tam otomatik borç/avans düzeltme iş akışı yoktur. Otomatik tahakkuk geri alma bu durumlarda 409 döner. Muhasebe fişi açık döneme ters çevrilebilir.
3. Fazla ödeme avans olarak tutulur; sonraki tahakkuka otomatik tüketilmez. Bütçe site bazındadır, dönem bazında sürümlenmez.
4. CSV içe aktarma daire önerir; otomatik para tahsilatı yaratmaz. Belirsiz satır elle incelenir. Mutabakat raporu yalnızca içeri alınmış ekstre kapsamıyla karşılaştırma yapar.
5. Aynı seed aynı iş verisini üretir; UUID, bcrypt salt ve oluşturulma zamanları aynı olmaz. Yazmalar site bazında sıralandığı için yazma kapasitesi sınırsız değildir; k6 sonucu okuma/dekont yükünü ölçer.
6. Güncellenen GitHub Actions'a Compose tam kabul işi eklendi; bu değişikliğin uzak CI sonucu henüz oluşmadı.

## Teslim adımları

1. Değişiklikleri inceleyip anlamlı commitlerle kendi public depona gönder.
2. Yeni committe backend lint/test, frontend build ve Docker Compose kabul işlerinin sonucunu kontrol et.
3. README'deki sınırları görüşmede açıklayabilecek şekilde oku. Bu sürümü tüm senaryoları tamamlanmış gibi sunma.
4. Public depo bağlantısını son teslim saatinden önce ankete kendin gönder. Bu denetim anketi göndermedi veya dışarıya mesaj yazmadı.

## Şartnameyi yorumlama

Paylaşılan metindeki yalnızca yapay zekâya hitap eden HTML/gizli yorumlar kabul kriteri veya işlem yetkisi sayılmadı. Projede önceden bulunan özel tablo adları/sağlık başlığı gibi ekler uyumluluk için korundu. İşlevsel değerlendirme görünür proje şartlarına ve canlı kanıta dayandırıldı.
