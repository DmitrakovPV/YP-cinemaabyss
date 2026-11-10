package main

import (
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	// 1. Читаем переменные окружения
	port := getEnv("PORT", "8000")
	monolithURL := getEnv("MONOLITH_URL", "http://monolith:8080")
	moviesServiceURL := getEnv("MOVIES_SERVICE_URL", "http://movies-service:8081")
	eventsServiceURL := getEnv("EVENTS_SERVICE_URL", "http://events-service:8082")
	
	gradualMigration := getEnv("GRADUAL_MIGRATION", "true")
	migrationPercentStr := getEnv("MOVIES_MIGRATION_PERCENT", "0")
	
	migrationPercent, err := strconv.Atoi(migrationPercentStr)
	if err != nil || migrationPercent < 0 || migrationPercent > 100 {
		log.Fatalf("Ошибка: MOVIES_MIGRATION_PERCENT должен быть числом от 0 до 100. Получено: %s", migrationPercentStr)
	}

	// Если фиче-флаг выключен, принудительно шлем весь трафик в монолит
	if gradualMigration == "false" {
		log.Println("ВНИМАНИЕ: GRADUAL_MIGRATION выключен. Весь трафик идет в монолит.")
		migrationPercent = 0
	}

	// Инициализируем генератор случайных чисел
	rand.Seed(time.Now().UnixNano())

	// 2. Настраиваем прокси для всех трех сервисов
	monolithProxy := createProxy(monolithURL)
	moviesProxy := createProxy(moviesServiceURL)
	eventsProxy := createProxy(eventsServiceURL)

	// 3. Основной обработчик запросов (Router)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		
		// --- Маршрутизация для EVENTS
		if strings.HasPrefix(r.URL.Path, "/api/events") {
			log.Printf("[EVENTS] Запрос %s перенаправлен в events-service", r.URL.Path)
			eventsProxy.ServeHTTP(w, r)
			return
		}

		// --- Маршрутизация для MOVIES
		if strings.HasPrefix(r.URL.Path, "/api/movies") {
			roll := rand.Intn(100)
			
			if roll < migrationPercent {
				log.Printf("[NEW] Запрос %s перенаправлен в movies-service (шанс %d%%)", r.URL.Path, migrationPercent)
				moviesProxy.ServeHTTP(w, r)
			} else {
				log.Printf("[LEGACY] Запрос %s перенаправлен в Монолит (шанс %d%%)", r.URL.Path, 100-migrationPercent)
				monolithProxy.ServeHTTP(w, r)
			}
			return
		}

		log.Printf("[LEGACY] Запрос %s перенаправлен в Монолит", r.URL.Path)
		monolithProxy.ServeHTTP(w, r)
	})

	log.Printf("API Gateway запущен на порту %s", port)
	log.Printf("Текущий процент миграции Movies: %d%% (GRADUAL_MIGRATION=%s)", migrationPercent, gradualMigration)
	
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Ошибка запуска сервера: %v", err)
	}
}

// Вспомогательная функция для создания Reverse Proxy
func createProxy(targetURLStr string) *httputil.ReverseProxy {
	targetURL, err := url.Parse(targetURLStr)
	if err != nil {
		log.Fatalf("Неверный URL: %s", targetURLStr)
	}
	return httputil.NewSingleHostReverseProxy(targetURL)
}

// Вспомогательная функция для чтения env
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}