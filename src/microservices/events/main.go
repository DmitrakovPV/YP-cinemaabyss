package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

var (
	kafkaBrokers = []string{"kafka:9092"}
)

func main() {
	// 1. Получаем настройки из окружения
	port := getEnv("PORT", "8082")
	brokersEnv := getEnv("KAFKA_BROKERS", "kafka:9092")
	kafkaBrokers = strings.Split(brokersEnv, ",")

	log.Printf("Запуск Events Service на порту %s", port)
	log.Printf("Подключение к Kafka: %v", kafkaBrokers)

	// 2. Запускаем Consumer в фоновом режиме
	go startConsumer()

	// 3. Настраиваем HTTP-роутер для Producer-а
	http.HandleFunc("/api/events/user", handleEvent("user-events"))
	http.HandleFunc("/api/events/payment", handleEvent("payment-events"))
	http.HandleFunc("/api/events/movie", handleEvent("movie-events"))
	http.HandleFunc("/api/events/health", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": true,
        "service": "events-service",
    })
})

	// 4. Запускаем HTTP-сервер
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Ошибка запуска HTTP сервера: %v", err)
	}
}

// Обработчик HTTP-запросов, который создает событие в Kafka
func handleEvent(topic string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		// Читаем тело запроса
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Ошибка чтения тела запроса", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Проверяем, что это валидный JSON
		if !json.Valid(body) {
			http.Error(w, "Невалидный JSON", http.StatusBadRequest)
			return
		}

		// Создаем Writer для отправки в Kafka
		writer := &kafka.Writer{
			Addr:     kafka.TCP(kafkaBrokers...),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		}
		defer writer.Close()

		// Отправляем сообщение
		msg := kafka.Message{
			Key:   []byte(time.Now().Format(time.RFC3339)),
			Value: body,
		}

		err = writer.WriteMessages(r.Context(), msg)
		if err != nil {
			log.Printf("[ОШИБКА] Не удалось отправить сообщение в топик %s: %v", topic, err)
			http.Error(w, "Ошибка отправки в Kafka", http.StatusInternalServerError)
			return
		}

		log.Printf("[PRODUCER] Событие отправлено в топик [%s]: %s", topic, string(body))
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"event_created"}`))
	}
}

// Функция для чтения сообщений из Kafka (Consumer)
func startConsumer() {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  kafkaBrokers,
		GroupID:  "events-service-group",
		Topic:    "user-events",
		MinBytes: 10e3,   
		MaxBytes: 10e6,
	})
	defer reader.Close()

	log.Println("[CONSUMER] Запущен слушатель Kafka...")
	
	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("[CONSUMER ERROR] %v", err)
			continue
		}
		log.Printf("[CONSUMER] Получено сообщение из топика [%s], key: %s, value: %s", 
			m.Topic, string(m.Key), string(m.Value))
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}