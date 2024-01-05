package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fur1ouswolf/nats_streaming/internal/app"
	"github.com/fur1ouswolf/nats_streaming/internal/cache"
	"github.com/fur1ouswolf/nats_streaming/internal/models"
	"github.com/fur1ouswolf/nats_streaming/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nats-io/stan.go"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type App struct {
	logger *slog.Logger
	server *http.Server
	db     repository.Repository
	cache  *cache.Cache
	sc     stan.Conn
}

func NewApp(repo repository.Repository, logger *slog.Logger, c *cache.Cache) (app.App, error) {
	a := &App{
		logger: logger,
		db:     repo,
		cache:  c,
	}
	var err error
	a.sc, err = stan.Connect(os.Getenv("NATS_CLUSTER"), os.Getenv("NATS_CLIENT"), stan.NatsURL(os.Getenv("NATS_URL")))
	if err != nil {
		logger.Error(fmt.Sprint("NATS connection error:", err.Error()))
		return nil, err
	}
	err = a.Subscribe(os.Getenv("NATS_CHANNEL"))
	if err != nil {
		logger.Error(fmt.Sprint("NATS subscription error:", err.Error()))
		return nil, err
	}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/{uid}", a.GetOrder)

	a.server = &http.Server{
		Addr:    os.Getenv("APP_HOST") + ":" + os.Getenv("APP_PORT"),
		Handler: r,
	}

	a.logger.Info("Recovering cache...")
	err = c.Recover(repo)
	if err != nil {
		return nil, err
	}

	return a, nil
}

func (a *App) Subscribe(channel string) error {
	_, err := a.sc.Subscribe(channel, func(m *stan.Msg) {
		var order models.Order
		err := json.Unmarshal(m.Data, &order)
		if err != nil {
			return
		}
		_, ok := a.cache.Get(order.OrderUID)
		if ok {
			return
		}

		a.cache.Set(order.OrderUID, order)
		err = a.db.InsertOrder(order)
		if err != nil {
			a.logger.Error(fmt.Sprint("Insert order error:", err.Error()))
		}
		a.logger.Info(fmt.Sprintf("New order inserted: %s", order.OrderUID))
	},
	)

	if err != nil {
		return err
	}

	return nil
}

func (a *App) GetOrder(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")
	w.Header().Set("Content-Type", "application/json")

	order, ok := a.cache.Get(uid)
	if !ok {
		var err error
		order, err = a.db.GetOrderByUID(uid)
		if err != nil {
			http.Error(w, "Order not found", http.StatusNotFound)
			return
		}
		a.cache.Set(order.OrderUID, order)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(order)

}

func (a *App) Start() error {
	go func() {
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logger.Error(fmt.Sprint("Server error:", err.Error()))
		}
	}()

	a.logger.Info("Server started")
	return nil
}

func (a *App) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		a.logger.Error(fmt.Sprint("Server shutdown failed:", err.Error()))
		return err
	}

	if err := a.sc.Close(); err != nil {
		a.logger.Error(fmt.Sprint("NATS connection close error:", err.Error()))
		return err
	}

	a.logger.Info("Server stopped")
	return nil
}
