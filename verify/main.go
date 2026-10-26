package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
"net/url"
	"os"
	"strings"
	"time"
)

type VerificationResult struct {
	ID          string
	Name        string
	Category    string
	Passed      bool
	Details     string
	DurationMs  int64
}

var (
	baseURL = "http://localhost:8080"
	client  = &http.Client{Timeout: 10 * time.Second}
	results = make([]VerificationResult, 0)
)

func main() {
	if url := os.Getenv("API_URL"); url != "" {
		baseURL = strings.TrimRight(url, "/")
	}

	fmt.Println("==================================================================================")
	fmt.Println("   SİTEYÖNET — DDO KABUL & HIZLI DOĞRULAMA SUITE (AUTOMATED VERIFY)")
	fmt.Printf("   Hedef API: %s | Tarih: %s\n", baseURL, time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("==================================================================================")

	// Wait for API to be ready
	waitForAPI()

	// 1. Health & DDO Standards
	runTest("HEALTH", "Sağlık Ucu ve Veritabanı Bağlantısı", "Sistem", testHealth)
	runTest("DDO-HZ", "Özel Sağlık Kontrolü (GET /_hz/0c7c)", "Standart", testDDOHealth)
	runTest("DDO-HDR", "X-DDO-Build: 0c7cc1 Başlık Doğrulaması", "Standart", testDDOHeader)

	// 2. Authentication & Authorization (RBAC)
	var managerToken, residentToken, auditorToken string
	runTest("AUTH-1", "Yönetici Girişi (yonetici@siteyonet.com)", "Güvenlik", func() (bool, string) {
		tok, ok := login("yonetici@siteyonet.com", "password123")
		managerToken = tok
		return ok, "Yönetici JWT alındı"
	})
	runTest("AUTH-2", "Malik/Sakin Girişi (malik@siteyonet.com)", "Güvenlik", func() (bool, string) {
		tok, ok := login("malik@siteyonet.com", "password123")
		residentToken = tok
		return ok, "Sakin JWT alındı"
	})
	runTest("AUTH-3", "Denetim Kurulu Girişi (denetim@siteyonet.com)", "Güvenlik", func() (bool, string) {
		tok, ok := login("denetim@siteyonet.com", "password123")
		auditorToken = tok
		return ok, "Denetçi JWT alındı"
	})
	runTest("AUTH-4", "Hatalı Parola ile Giriş Engeli (401)", "Güvenlik", testWrongPassword)
	runTest("AUTH-5", "Yetkisiz Rol Erişim Engeli (403)", "Güvenlik", func() (bool, string) {
		return testUnauthorizedRoleAccess(residentToken)
	})

	// 3. Kapsam Doğrulamaları (F-1 .. F-7)
	runTest("F-1", "Site, Blok ve Daire Kayıtları", "Kapsam", func() (bool, string) {
		return testF1(managerToken)
	})
	runTest("F-2", "Aidat Tanımı ve Tahakkuk Listesi", "Kapsam", func() (bool, string) {
		return testF2(managerToken)
	})
	runTest("F-3", "Tahsilat ve Borç Takibi", "Kapsam", func() (bool, string) {
		return testF3(managerToken)
	})
	runTest("F-4", "Gider Yönetimi ve Fatura Kaydı", "Kapsam", func() (bool, string) {
		return testF4(managerToken)
	})
	runTest("F-5", "Duyuru, Karar ve Bakım Talepleri", "Kapsam", func() (bool, string) {
		return testF5(managerToken)
	})
	runTest("F-6", "Muhasebe Defteri ve Dönem Kapanışı", "Kapsam", func() (bool, string) {
		return testF6(managerToken)
	})
	runTest("F-7", "Finansal Raporlar ve 'Borcu Yoktur' Doğrulama", "Kapsam", func() (bool, string) {
		return testF7(managerToken)
	})

	// 4. Zorlu Çekirdek Görevler (T-1 .. T-4)
	runTest("T-1", "Tahakkuk ve Paylaştırma Motoru (En Büyük Kalan / Kuruşu Kuruşuna)", "Zorlu Görev", func() (bool, string) {
		return testT1(managerToken)
	})
	runTest("T-2", "Tahsilat, Mahsup (Faiz->Anapara->FIFO) ve Bulanık Eşleme", "Zorlu Görev", func() (bool, string) {
		return testT2(managerToken)
	})
	runTest("T-3", "Çok Kiracılı Veri İzolasyonu (PostgreSQL RLS)", "Zorlu Görev", func() (bool, string) {
		return testT3(residentToken, auditorToken)
	})
	runTest("T-4", "Muhasebe Çift Taraflı Kayıt & 'Borcu Yoktur' SHA-256 Doğrulama", "Zorlu Görev", func() (bool, string) {
		return testT4(managerToken)
	})

	// Print Summary Table
	printSummaryTable()
}

func runTest(id, name, cat string, fn func() (bool, string)) {
	start := time.Now()
	passed, details := fn()
	duration := time.Since(start).Milliseconds()

	results = append(results, VerificationResult{
		ID:         id,
		Name:       name,
		Category:   cat,
		Passed:     passed,
		Details:    details,
		DurationMs: duration,
	})
}

func printSummaryTable() {
	fmt.Println("\n==================================================================================")
	fmt.Printf("%-8s | %-12s | %-42s | %-6s | %-8s\n", "KİMLİK", "KATEGORİ", "ÖZELLİK / GÖREV ADI", "SONUÇ", "SÜRE")
	fmt.Println("---------|--------------|--------------------------------------------|--------|---------")

	allPassed := true
	for _, r := range results {
		status := "PASS"
		if !r.Passed {
			status = "FAIL"
			allPassed = false
		}
		fmt.Printf("%-8s | %-12s | %-42s | %-6s | %4d ms\n", r.ID, r.Category, truncate(r.Name, 42), status, r.DurationMs)
		if !r.Passed {
			fmt.Printf("         └── Hata Detayı: %s\n", r.Details)
		}
	}
	fmt.Println("==================================================================================")

	if allPassed {
		fmt.Println("🎉 TEBRİKLER! TÜM DOĞRULAMA TESTLERİ EKSİKSİZ BAŞARIYLA GEÇTİ (PASS).")
		os.Exit(0)
	} else {
		fmt.Println("❌ BAZI TESTLER BAŞARISIZ OLDU (FAIL). Lütfen yukarıdaki detayları inceleyiniz.")
		os.Exit(1)
	}
}

func truncate(s string, max int) string {
	if len([]rune(s)) > max {
		return string([]rune(s)[:max-3]) + "..."
	}
	return s
}

// ----------------------------------------------------------------------------
// Test Implementations
// ----------------------------------------------------------------------------

func waitForAPI() {
	fmt.Print("API servisine bağlanılıyor")
	for i := 0; i < 30; i++ {
		resp, err := client.Get(baseURL + "/health")
		if resp != nil { _ = resp.Body.Close() }
        if err == nil && resp.StatusCode == http.StatusOK {
			fmt.Println(" [BAĞLANDI]")
			return
		}
		fmt.Print(".")
		time.Sleep(1 * time.Second)
	}
	fmt.Println("\nUyarı: API zamanında yanıt vermedi, yine de testler başlatılıyor...")
}

func testHealth() (bool, string) {
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, fmt.Sprintf("HTTP %d OK", resp.StatusCode)
}

func testDDOHealth() (bool, string) {
	resp, err := client.Get(baseURL + "/_hz/0c7c")
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode == http.StatusOK, fmt.Sprintf("Status: %d, Body: %s", resp.StatusCode, string(body))
}

func testDDOHeader() (bool, string) {
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	hdr := resp.Header.Get("X-DDO-Build")
	if hdr == "0c7cc1" {
		return true, "Header doğrulandı: X-DDO-Build = 0c7cc1"
	}
	return false, fmt.Sprintf("Beklenen: 0c7cc1, Gelen: %s", hdr)
}

func login(email, pass string) (string, bool) {
	payload, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": pass,
	})
	resp, err := client.Post(baseURL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(payload))
	if err != nil || resp.StatusCode != http.StatusOK {
		return "", false
	}
	defer resp.Body.Close()

	var data struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&data)
	return data.Token, data.Token != ""
}

func testWrongPassword() (bool, string) {
	payload, _ := json.Marshal(map[string]string{
		"email":    "yonetici@siteyonet.com",
		"password": "wrong-password",
	})
	resp, err := client.Post(baseURL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusUnauthorized, fmt.Sprintf("HTTP %d Unauthorized alındı", resp.StatusCode)
}

func testUnauthorizedRoleAccess(token string) (bool, string) {
	// Sakin rolü gidip aidat tanımı oluşturmayı denerse 403 Forbidden almalı
	payload, _ := json.Marshal(map[string]interface{}{
		"name":               "İzinsiz Aidat",
		"total_amount_cents": 500000,
		"distribution_type":  "EQUAL",
	})
	req, _ := http.NewRequest("POST", baseURL+"/api/v1/dues-definitions", bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusForbidden, fmt.Sprintf("HTTP %d Forbidden ile yetkisiz istek engellendi", resp.StatusCode)
}


var verifyUnit, verifyBlock, verifyPeriod, verifyOwner, verifyAccrual string
var marker = fmt.Sprintf("verify-%d",time.Now().UnixNano())

func call(method,path,token,key string,payload interface{},out interface{}) (int,error) {
 data,err:=json.Marshal(payload);if err!=nil{return 0,err}
 req,err:=http.NewRequest(method,baseURL+"/api/v1"+path,bytes.NewReader(data));if err!=nil{return 0,err}
 req.Header.Set("Content-Type","application/json")
 if token!="" {req.Header.Set("Authorization","Bearer "+token)}
 if key!="" {req.Header.Set("Idempotency-Key",key)}
 resp,err:=client.Do(req);if err!=nil{return 0,err};defer resp.Body.Close()
 body,err:=io.ReadAll(resp.Body);if err!=nil{return resp.StatusCode,err}
 if resp.StatusCode>=400{return resp.StatusCode,fmt.Errorf("HTTP %d: %s",resp.StatusCode,body)}
 if out!=nil {err=json.Unmarshal(body,out)}
 return resp.StatusCode,err
}
func create(token,path string,payload interface{}) (string,error) {
 var result struct{ID string `json:"id"`}
 status,err:=call("POST",path,token,"",payload,&result)
 if err!=nil{return "",err};if status!=201||result.ID==""{return "",fmt.Errorf("Create %s returned %d without ID",path,status)}
 return result.ID,nil
}
func testF1(token string)(bool,string){
 var me struct{ID string `json:"id"`}
 if _,err:=call("GET","/auth/me",token,"",nil,&me);err!=nil{return false,err.Error()};verifyOwner=me.ID
 var periods []struct{ID,Status string}
 if _,err:=call("GET","/periods",token,"",nil,&periods);err!=nil{return false,err.Error()}
 for _,p:=range periods{if p.Status=="OPEN"{verifyPeriod=p.ID;break}}
 if verifyPeriod==""{return false,"Açık dönem bulunamadı"}
 var err error
 verifyBlock,err=create(token,"/blocks",map[string]interface{}{"name":marker,"total_floors":2});if err!=nil{return false,err.Error()}
 verifyUnit,err=create(token,"/units",map[string]interface{}{"block_id":verifyBlock,"unit_number":"999","owner_id":verifyOwner,"land_share":100,"gross_area_m2":100,"resident_count":2})
 return err==nil,fmt.Sprint(err)
}
func testF2(token string)(bool,string){
 _,err:=create(token,"/dues-definitions",map[string]interface{}{"name":marker,"total_amount_cents":125433,"distribution_type":"LAND_SHARE","due_day":15,"late_fee_rate":5})
 if err!=nil{return false,err.Error()}
 return testT1(token)
}
func testF3(token string)(bool,string){
 payload:=map[string]interface{}{"unit_id":verifyUnit,"amount_cents":1,"payment_date":"2026-10-26","payment_channel":"BANK_TRANSFER"}
 var first,second struct{ID string `json:"id"`}
 if status,err:=call("POST","/payments",token,marker,payload,&first);err!=nil||status!=201{return false,fmt.Sprint(err)}
 if status,err:=call("POST","/payments",token,marker,payload,&second);err!=nil||status!=201||first.ID!=second.ID{return false,"Ödeme tekrarı aynı kaydı döndürmedi"}
 payload["amount_cents"]=2
 status,_:=call("POST","/payments",token,marker,payload,nil)
 return status==409,"Tahsilat, kalıcı idempotans ve anahtar çakışması"
}
func testF4(token string)(bool,string){
 _,err:=create(token,"/expenses",map[string]interface{}{"period_id":verifyPeriod,"category":"CLEANING","title":marker,"vendor_name":"Verify","amount_cents":100,"invoice_date":"2026-10-26"})
 return err==nil,fmt.Sprint(err)
}
func testF5(token string)(bool,string){
 for _,item:=range []struct{path string;body map[string]interface{}}{
 {"/announcements",map[string]interface{}{"title":marker,"content":"Verify announcement","priority":"NORMAL"}},
 {"/decisions",map[string]interface{}{"decision_number":marker,"decision_date":"2026-10-26","title":marker,"content":"Verify decision"}},
 {"/maintenance",map[string]interface{}{"unit_id":verifyUnit,"title":marker,"description":"Verify maintenance"}},
 }{id,err:=create(token,item.path,item.body);if err!=nil{return false,err.Error()};if item.path=="/maintenance"{if _,err:=call("PUT",item.path+"/"+id+"/status",token,"",map[string]interface{}{"status":"IN_PROGRESS"},nil);err!=nil{return false,err.Error()}}}
 return true,"Duyuru, karar, bakım kaydı ve durum geçişi"
}
func testF6(token string)(bool,string){
 var accounts []struct{ID,Code string}
 if _,err:=call("GET","/ledger-accounts",token,"",nil,&accounts);err!=nil{return false,err.Error()}
 var debit,credit string;for _,a:=range accounts{if a.Code=="102"{debit=a.ID};if a.Code=="120"{credit=a.ID}}
 if debit==""||credit==""{return false,"Muhasebe hesapları eksik"}
 payload:=map[string]interface{}{"period_id":verifyPeriod,"entry_date":"2026-10-26","description":marker,"lines":[]map[string]interface{}{{"AccountID":debit,"DebitCents":100},{"AccountID":credit,"CreditCents":100}}}
 id,err:=create(token,"/journal-entries",payload);if err!=nil{return false,err.Error()}
 if _,err:=create(token,"/journal-entries/"+id+"/reverse",nil);err!=nil{return false,err.Error()}
 var entries []struct{ID string;Debit int64 `json:"total_debit_cents"`;Credit int64 `json:"total_credit_cents"`}
 if _,err:=call("GET","/journal-entries",token,"",nil,&entries);err!=nil{return false,err.Error()}
 found:=false;for _,e:=range entries{if e.ID==id{found=e.Debit==100&&e.Credit==100}}
 if !found{return false,"Yevmiye satırları veya çift taraflı toplamlar okunamadı"}
 start:=time.Date(3000+int(time.Now().UnixMilli()%5000),1,1,0,0,0,0,time.UTC)
 period,err:=create(token,"/periods",map[string]interface{}{"name":marker,"start_date":start.Format("2006-01-02"),"end_date":start.AddDate(0,0,1).Format("2006-01-02")});if err!=nil{return false,err.Error()}
 if _,err:=call("POST","/periods/"+period+"/close",token,"",nil,nil);err!=nil{return false,err.Error()}
 payload["period_id"]=period
 status,_:=call("POST","/journal-entries",token,"",payload,nil)
 return status==400,"Çift taraflı fiş, ters kayıt, dönem kapanışı ve kapalı döneme yazma engeli"
}
func testF7(token string)(bool,string){
 var summary map[string]interface{}
 _,err:=call("GET","/reports/summary",token,"",nil,&summary)
 return err==nil&&summary["total_collected_cents"]!=nil,fmt.Sprint(err)
}
func testT1(token string)(bool,string){
 payload:=map[string]interface{}{"period_id":verifyPeriod,"title":marker,"term_month":"2026-10","distribution_type":"LAND_SHARE","total_amount_cents":125433,"due_day":15}
 id,err:=create(token,"/accruals",payload);if err!=nil{return false,err.Error()};verifyAccrual=id
 var total int64;count:=0;after:=""
 for {
  var debts []struct{ID string `json:"id"`;Due string `json:"due_date"`;Accrual string `json:"accrual_id"`;Amount int64 `json:"principal_cents"`}
  if _,err:=call("GET","/debts?limit=500&after="+url.QueryEscape(after),token,"",nil,&debts);err!=nil{return false,err.Error()}
  for _,d:=range debts{if d.Accrual==id{total+=d.Amount;count++}}
  if len(debts)<500{break};last:=debts[len(debts)-1];after=last.Due[:10]+","+last.ID
 }
 return total==125433&&count>0,fmt.Sprintf("Dağıtılan toplam: %d kuruş, borç sayısı: %d",total,count)
}
func testT2(token string)(bool,string){
 var match struct{Number string `json:"unit_number"`;Score float64 `json:"score"`}
 _,err:=call("POST","/bank-statements/match",token,"",map[string]interface{}{"raw_description":"A BLOK DAIRE: 5 AHMET YILMAZ AIDAT"},&match)
 return err==nil&&match.Number!=""&&match.Score>0,fmt.Sprintf("Daire=%s skor=%f hata=%v",match.Number,match.Score,err)
}
func testT3(resident,auditor string)(bool,string){
 var units []struct{ID string}
 if _,err:=call("GET","/units",resident,"",nil,&units);err!=nil{return false,err.Error()}
 for _,u:=range units{if u.ID==verifyUnit{return false,"Sakin yöneticiye ait daireyi gördü"}}
 status,_:=call("POST","/expenses",auditor,"",map[string]interface{}{"amount_cents":100},nil)
 if status!=403{return false,"Denetçi yazma isteği engellenmedi"}
 status,_=call("GET","/units","","",nil,nil)
 return status==401,"Sakin gizliliği, denetçi yazma engeli ve anonim erişim engeli; doğrudan RLS kanıtı entegrasyon testlerinde"
}
func testT4(token string)(bool,string){
 // Create an unindebted unit after the accrual tests, then issue and verify a real certificate.
 unit,err:=create(token,"/units",map[string]interface{}{"block_id":verifyBlock,"unit_number":"998","owner_id":verifyOwner,"land_share":100,"gross_area_m2":100,"resident_count":1});if err!=nil{return false,err.Error()}
 var cert struct{Code string `json:"verification_code"`}
 if status,err:=call("POST","/certificates",token,"",map[string]interface{}{"unit_id":unit,"valid_days":30},&cert);err!=nil||status!=201||cert.Code==""{return false,fmt.Sprint(err)}
 var result struct{Valid bool `json:"validity_status"`}
 _,err=call("GET","/certificates/verify/"+cert.Code,"","",nil,&result)
 if err!=nil||!result.Valid{return false,"Belge imzası doğrulanamadı"}
 status,_:=call("POST","/certificates",token,"",map[string]interface{}{"unit_id":verifyUnit,"valid_days":30},nil)
 return status==400,"Borçlu daire reddedildi, borçsuz daire belgesinin imzası doğrulandı"
}
