package cache

import (
	"github.com/fur1ouswolf/nats_streaming/internal/models"
	"github.com/fur1ouswolf/nats_streaming/internal/repository"
	"sync"
)

type Cache struct {
	orders map[string]models.Order
	mutex  *sync.RWMutex
}

func NewCache() *Cache {
	return &Cache{
		orders: make(map[string]models.Order),
		mutex:  &sync.RWMutex{},
	}
}

func (c *Cache) Recover(repo repository.Repository) error {
	var orders []models.Order
	orders, err := repo.GetOrders()
	if err != nil {
		return err
	}

	for _, order := range orders {
		c.orders[order.OrderUID] = order
	}

	return nil
}

func (c *Cache) GetAll() ([]models.Order, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	var orders []models.Order
	for _, order := range c.orders {
		orders = append(orders, order)
	}
	return orders, nil
}

func (c *Cache) Get(key string) (models.Order, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	v, ok := c.orders[key]
	return v, ok
}

func (c *Cache) Set(key string, value models.Order) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.orders[key] = value
}
