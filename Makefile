.PHONY: all build up down verify reset-demo test load lint help

all: build up

help:
	@echo "SiteYonet Kullanılabilir Komutlar:"
	@echo "  make up          - Tüm sistemi Docker ile ayağa kaldırır (Frontend: 3000, Backend: 8080)"
	@echo "  make build       - Konteyner imajlarını derler"
	@echo "  make verify      - Tüm F-1..F-7 ve T-1..T-4 görevlerini test eder ve PASS/FAIL tablosu basar"
	@echo "  make reset-demo  - Veritabanını sıfırlayıp taze demo verisini seed eder"
	@echo "  make test        - Go birim ve entegrasyon testlerini çalıştırır (25+ test)"
	@echo "  make load        - k6 yük testini çalıştırır"
	@echo "  make down        - Tüm servisleri durdurur"

build:
	docker compose build

up:
	docker compose up -d --build

down:
	docker compose down -v

# Zorunlu Hızlı Doğrulama Komutu (Bölüm 10)
verify:
	docker compose --profile verify run --rm verify

# Zorunlu Sıfırlama Komutu (Bölüm 10)
reset-demo:
	docker compose down -v
	docker compose up -d db
	@echo "Veritabanının hazır olması bekleniyor..."
	sleep 4
	docker compose up -d backend
	@echo "Backend ve migrasyonların tamamlanması bekleniyor..."
	sleep 5
	docker compose run --rm simulator
	docker compose up -d frontend
	@echo "✅ Demo verileri ilk seed haline başarıyla döndürüldü!"

test:
	cd backend && go test -v -race ./tests/...

lint:
	cd backend && golangci-lint run ./...

load:
	k6 run k6/load-test.js
