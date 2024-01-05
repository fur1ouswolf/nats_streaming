package repository

import (
	"github.com/fur1ouswolf/nats_streaming/internal/models"
)

type Repository interface {
	GetOrders() ([]models.Order, error)
	GetOrderByUID(orderUID string) (models.Order, error)
	GetItemsByUID(orderUID string) ([]models.Item, error)
	GetPaymentByUID(orderUID string) (models.Payment, error)
	InsertItem(item models.Item) error
	InsertOrder(order models.Order) error
	InsertPayment(payment models.Payment) error
	InsertOrderItems(orderUID string, chrtIDs []int) error
}
