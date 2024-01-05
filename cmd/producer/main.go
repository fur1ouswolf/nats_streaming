package main

import (
	"encoding/json"
	"fmt"
	"github.com/fur1ouswolf/nats_streaming/internal/models"
	"github.com/joho/godotenv"
	"github.com/nats-io/stan.go"
	"os"
	"time"
)

func main() {
	if err := godotenv.Load("./.env"); err != nil {
		panic(err)
	}

	files, err := OSReadDir("cmd/producer/orders")
	if err != nil {
		panic(err)
	}
	var orders []models.Order
	for _, file := range files {
		f, err := os.ReadFile(fmt.Sprintf("./cmd/producer/orders/%s", file))
		if err != nil {
			panic(err)
		}
		var order models.Order
		err = json.Unmarshal(f, &order)
		if err != nil {
			panic(err)
		}
		orders = append(orders, order)
	}

	sc, err := stan.Connect(os.Getenv("NATS_CLUSTER"), "wb-producer", stan.NatsURL(os.Getenv("NATS_URL")))
	if err != nil {
		panic(err)
	}
	defer func(sc stan.Conn) {
		err := sc.Close()
		if err != nil {
			panic(err)
		}
	}(sc)

	for _, order := range orders {
		b, err := json.Marshal(order)
		if err != nil {
			panic(err)
		}
		err = sc.Publish(os.Getenv("NATS_CHANNEL"), b)
		if err != nil {
			panic(err)
		}
		fmt.Println("Order published: ", order.OrderUID)
		time.Sleep(time.Second * 2)
	}

	sub, err := sc.Subscribe(os.Getenv("NATS_CHANNEL"), func(m *stan.Msg) {
		fmt.Println("Received a message: ", string(m.Data))
	})
	if err != nil {
		panic(err)
	}
	defer func(sub stan.Subscription) {
		err := sub.Unsubscribe()
		if err != nil {
			panic(err)
		}
	}(sub)

	time.Sleep(time.Second * 10)

}

func OSReadDir(root string) ([]string, error) {
	var files []string
	f, err := os.Open(root)
	if err != nil {
		return files, err
	}
	fileInfo, err := f.Readdir(-1)
	err = f.Close()
	if err != nil {
		return nil, err
	}
	if err != nil {
		return files, err
	}

	for _, file := range fileInfo {
		files = append(files, file.Name())
	}
	return files, nil
}
