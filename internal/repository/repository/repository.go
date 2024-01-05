package repository

import (
	"context"
	"fmt"
	"github.com/fur1ouswolf/nats_streaming/internal/models"
	"github.com/fur1ouswolf/nats_streaming/internal/repository"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"log/slog"
	"os"
	"time"
)

type PostgresRepository struct {
	DB     *sqlx.DB
	Logger *slog.Logger
}

func NewRepository(l *slog.Logger) (repository.Repository, error) {
	db, err := sqlx.Connect("pgx", os.Getenv("DB_URL"))
	if err != nil {
		return nil, err
	}

	return &PostgresRepository{
		DB:     db,
		Logger: l,
	}, nil
}

func (r *PostgresRepository) GetOrders() ([]models.Order, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var orders []models.Order

	err := r.DB.SelectContext(ctx, &orders, "SELECT order_uid, track_number, entry, locale, internal_signature, customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard FROM orders")
	if err != nil {
		return nil, err
	}
	for i, order := range orders {
		payment, err := r.GetPaymentByUID(order.OrderUID)
		if err != nil {
			r.Logger.Error(fmt.Sprint("Getting payment error:", err.Error()))
			return nil, err
		}
		orders[i].Payment = payment

		items, err := r.GetItemsByUID(order.OrderUID)
		if err != nil {
			r.Logger.Error(fmt.Sprint("Getting items error:", err.Error()))
			return nil, err
		}
		orders[i].Items = items
	}

	return orders, nil
}

func (r *PostgresRepository) GetOrderByUID(orderUID string) (models.Order, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var order models.Order
	err := r.DB.GetContext(ctx, &order, "SELECT order_uid, track_number, entry, locale, internal_signature, customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard FROM orders WHERE order_uid = $1", orderUID)
	if err != nil {
		//r.Logger.Error("Getting order error: ", err.Error())
		return models.Order{}, err
	}

	payment, err := r.GetPaymentByUID(orderUID)
	if err != nil {
		r.Logger.Error(fmt.Sprint("Getting payment error:", err.Error()))
		return models.Order{}, err
	}
	order.Payment = payment

	items, err := r.GetItemsByUID(orderUID)
	if err != nil {
		return models.Order{}, err
	}
	order.Items = items

	return order, nil
}

func (r *PostgresRepository) GetItemsByUID(orderUID string) ([]models.Item, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var items []models.Item
	err := r.DB.SelectContext(ctx, &items, `
		SELECT * FROM items
        	WHERE chrt_id IN (SELECT chrt_id FROM order_items WHERE order_uid = $1)`, orderUID)
	if err != nil {
		return nil, err
	}

	return items, nil
}

func (r *PostgresRepository) GetPaymentByUID(orderUID string) (models.Payment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var payment models.Payment
	err := r.DB.GetContext(ctx, &payment, "SELECT * FROM payments WHERE transaction = $1", orderUID)
	if err != nil {
		return models.Payment{}, err
	}

	return payment, nil
}

func (r *PostgresRepository) InsertItem(item models.Item) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.DB.ExecContext(ctx, `
			INSERT INTO items (chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			`, item.ChrtID, item.TrackNumber, item.Price, item.RID, item.Name, item.Sale, item.Size, item.TotalPrice, item.NmID, item.Brand, item.Status)

	if err != nil {
		r.Logger.Error(fmt.Sprint("Insert item error:", err.Error()))
		return err
	}

	return nil
}

func (r *PostgresRepository) InsertPayment(payment models.Payment) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.DB.ExecContext(ctx, `
			INSERT INTO payments (transaction, request_id, currency, provider, amount, payment_dt, bank, delivery_cost, goods_total, custom_fee)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`, payment.Transaction, payment.RequestID, payment.Currency, payment.Provider, payment.Amount, payment.PaymentDT, payment.Bank, payment.DeliveryCost, payment.GoodsTotal, payment.CustomFee)
	if err != nil {
		return err
	}

	return nil
}

func (r *PostgresRepository) InsertOrder(order models.Order) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.DB.ExecContext(ctx, `
			INSERT INTO orders (order_uid, track_number, entry, delivery, locale, internal_signature, customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`, order.OrderUID, order.TrackNumber, order.Entry, order.Delivery, order.Locale, order.InternalSignature, order.CustomerID, order.DeliveryService, order.ShardKey, order.SMID, order.DateCreated, order.OOFShard)
	if err != nil {
		return err
	}
	err = r.InsertPayment(order.Payment)
	if err != nil {
		return err
	}

	chrtIDs := make([]int, 0)
	for _, item := range order.Items {
		var chrtID int
		_ = r.DB.QueryRowContext(ctx, `
			SELECT chrt_id FROM items WHERE chrt_id = $1`, item.ChrtID).Scan(&chrtID)
		if chrtID == 0 {
			r.Logger.Error(fmt.Sprintf("Item with ChrtID %v not in DB!", item.ChrtID))
			err = r.InsertItem(item)
			if err != nil {
				return err
			}
		}
		chrtIDs = append(chrtIDs, item.ChrtID)
	}
	err = r.InsertOrderItems(order.OrderUID, chrtIDs)

	return nil
}

func (r *PostgresRepository) InsertOrderItems(orderUID string, chrtIDs []int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, id := range chrtIDs {
		_, err := r.DB.ExecContext(ctx, `
			INSERT INTO order_items (order_uid, chrt_id)
				VALUES ($1, $2)`, orderUID, id)
		if err != nil {
			return err
		}
	}

	return nil
}
