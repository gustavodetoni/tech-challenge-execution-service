package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Execution struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"order_id"`
	Status    string    `json:"status"`
	Notes     string    `json:"notes,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Event struct {
	ID         string    `json:"event_id"`
	Type       string    `json:"type"`
	OrderID    string    `json:"order_id"`
	OccurredAt time.Time `json:"occurred_at"`
	Data       any       `json:"data,omitempty"`
}
type App struct {
	collection *mongo.Collection
	logger     *slog.Logger
	publisher  *Publisher
}
type Publisher struct {
	conn     *amqp091.Connection
	channel  *amqp091.Channel
	exchange string
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(env("MONGODB_URL", "mongodb://localhost:27017")))
	if err != nil {
		panic(err)
	}
	if err = client.Ping(ctx, nil); err != nil {
		panic(err)
	}
	app := &App{collection: client.Database(env("MONGODB_DATABASE", "execution_service")).Collection("executions"), logger: logger, publisher: newPublisher(logger)}
	if app.publisher != nil {
		go app.consumePayments()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", app.health)
	mux.HandleFunc("POST /executions/{orderID}/start", app.start)
	mux.HandleFunc("POST /executions/{orderID}/finish", app.finish)
	mux.HandleFunc("GET /executions/{orderID}", app.get)
	logger.Info("execution service started", "port", env("PORT", "8083"))
	_ = http.ListenAndServe(":"+env("PORT", "8083"), withCorrelation(mux))
}
func (a *App) health(w http.ResponseWriter, _ *http.Request) {
	jsonResponse(w, 200, map[string]string{"status": "ok", "service": "execution-service"})
}
func (a *App) start(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("orderID")
	now := time.Now().UTC()
	e := Execution{ID: uuid.NewString(), OrderID: id, Status: "IN_PROGRESS", UpdatedAt: now}
	_, err := a.collection.UpdateOne(r.Context(), bson.M{"order_id": id}, bson.M{"$set": e, "$setOnInsert": bson.M{"created_at": now}}, options.UpdateOne().SetUpsert(true))
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "could not start execution"})
		return
	}
	a.publish(Event{ID: uuid.NewString(), Type: "ExecutionStarted", OrderID: id, OccurredAt: now, Data: e})
	jsonResponse(w, 202, e)
}
func (a *App) finish(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("orderID")
	now := time.Now().UTC()
	res, err := a.collection.UpdateOne(r.Context(), bson.M{"order_id": id}, bson.M{"$set": bson.M{"status": "FINISHED", "updated_at": now}})
	if err != nil || res.MatchedCount == 0 {
		jsonResponse(w, 404, map[string]string{"error": "execution not found"})
		return
	}
	a.publish(Event{ID: uuid.NewString(), Type: "ServiceOrderExecutionCompleted", OrderID: id, OccurredAt: now})
	jsonResponse(w, 200, map[string]string{"order_id": id, "status": "FINISHED"})
}
func (a *App) get(w http.ResponseWriter, r *http.Request) {
	var e Execution
	err := a.collection.FindOne(r.Context(), bson.M{"order_id": r.PathValue("orderID")}).Decode(&e)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": "execution not found"})
		return
	}
	jsonResponse(w, 200, e)
}
func (a *App) publish(e Event) {
	if a.publisher == nil {
		a.logger.Info("event queued", "event_type", e.Type, "order_id", e.OrderID)
		return
	}
	b, _ := json.Marshal(e)
	_ = a.publisher.channel.PublishWithContext(context.Background(), a.publisher.exchange, "execution."+e.Type, false, false, amqp091.Publishing{ContentType: "application/json", Body: b, MessageId: e.ID, Type: e.Type})
}
func (a *App) consumePayments() {
	ch, err := a.publisher.conn.Channel()
	if err != nil {
		return
	}
	defer ch.Close()
	_, _ = ch.QueueDeclare("execution-service-payments", true, false, false, false, nil)
	_ = ch.QueueBind("execution-service-payments", "billing.PaymentConfirmed", "tech-challenge.events", false, nil)
	msgs, err := ch.Consume("execution-service-payments", "execution-service", false, false, false, false, nil)
	if err != nil {
		return
	}
	for msg := range msgs {
		var e Event
		if json.Unmarshal(msg.Body, &e) == nil {
			now := time.Now().UTC()
			_, _ = a.collection.UpdateOne(context.Background(), bson.M{"order_id": e.OrderID}, bson.M{"$set": Execution{ID: uuid.NewString(), OrderID: e.OrderID, Status: "IN_PROGRESS", UpdatedAt: now}}, options.UpdateOne().SetUpsert(true))
		}
		_ = msg.Ack(false)
	}
}
func newPublisher(logger *slog.Logger) *Publisher {
	u := os.Getenv("RABBITMQ_URL")
	if u == "" {
		return nil
	}
	c, e := amqp091.Dial(u)
	if e != nil {
		logger.Error("rabbitmq unavailable", "error", e)
		return nil
	}
	ch, e := c.Channel()
	if e != nil {
		return nil
	}
	_ = ch.ExchangeDeclare("tech-challenge.events", "topic", true, false, false, false, nil)
	return &Publisher{conn: c, channel: ch, exchange: "tech-challenge.events"}
}
func withCorrelation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Correlation-ID")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Correlation-ID", id)
		next.ServeHTTP(w, r)
	})
}
func jsonResponse(w http.ResponseWriter, s int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s)
	_ = json.NewEncoder(w).Encode(v)
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
