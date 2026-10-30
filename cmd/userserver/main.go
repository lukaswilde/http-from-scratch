package main

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"httpServer/internal"
	"httpServer/internal/http1"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/joho/godotenv"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

type User struct {
	UserID int
	Name   string
}

func initTracer(ctx context.Context) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("user-service"),
		),
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 3. Create the TracerProvider with a batch span processor
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	// Sets W3C TraceContext propagation across HTTP headers
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

func main() {

	var users []User
	users = append(users, User{1, "Frank"})
	users = append(users, User{3, "Hank"})

	ctx := context.Background()
	tp, err := initTracer(ctx)
	if err != nil {
		log.Fatalf("failed to init tracer: %v", err)
	}

	defer func() {
		ctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		_ = tp.Shutdown(ctx)
	}()

	mux := http1.NewServeMux()

	mux.HandleFunc("/users", internal.TrackMetrics("/users", func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer("user-service")

		time.Sleep(100 * time.Millisecond)
		if r.Method == http.MethodGet {
			_, span := tracer.Start(r.Context(), "json-serialize", trace.WithSpanKind(trace.SpanKindInternal))
			usersJSON, err := json.Marshal(users)
			time.Sleep(50 * time.Millisecond)
			span.End()

			log.Println(usersJSON)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write(usersJSON)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, span := tracer.Start(r.Context(), "json-deserialize", trace.WithSpanKind(trace.SpanKindInternal))
			user := User{}
			err = json.Unmarshal(body, &user)
			time.Sleep(50 * time.Millisecond)
			span.End()

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			users = append(users, user)
			w.Header().Set("Location", "/users/"+strconv.Itoa(user.UserID))
			w.WriteHeader(http.StatusCreated)
		}
	}))
	mux.HandleFunc("/users/{id}", internal.TrackMetrics("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			id, err := strconv.Atoi(r.PathValue("id"))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}

			idx := slices.IndexFunc(users, func(u User) bool {
				return u.UserID == id
			})

			if idx == -1 {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			user := users[idx]
			usersJSON, err := json.Marshal(user)
			log.Println(usersJSON)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write(usersJSON)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics.WritePrometheus(w, true)
	})

	wrappedHandler := otelhttp.NewHandler(mux, "http-server")

	if err := godotenv.Load(); err != nil {
		log.Printf("Error loading .env file: %v. Falling back to system env variables", err)
	}

	port := os.Getenv("PORT")
	host := os.Getenv("HOST")
	addr := fmt.Sprintf("%s:%s", host, port)

	log.Printf("Starting server at %s", addr)
	if err := http1.ListenAndServe(addr, wrappedHandler); err != nil {
		panic(err)
	}
}
