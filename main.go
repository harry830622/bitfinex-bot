package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/bitfinexcom/bitfinex-api-go/v2"
	"github.com/bitfinexcom/bitfinex-api-go/v2/websocket"
)

func createClient(url string) *websocket.Client {
	key := os.Getenv("BFX_API_KEY")
	secret := os.Getenv("BFX_API_SECRET")
	param := websocket.NewDefaultParameters()
	param.URL = url
	client := websocket.NewWithParams(param).Credentials(key, secret)
	return client
}

func subFundingBook(ctx context.Context, client *websocket.Client, symbol string) {
	_, err := client.SubscribeBook(ctx, symbol, bitfinex.Precision0, bitfinex.FrequencyRealtime, 25)
	// _, err := client.SubscribeBook(ctx, symbol, "P0", "F0", 25)
	if err != nil {
		log.Fatalf("subscribing to funding book: %s", err)
	}
}

func main() {
	client := createClient("wss://api.bitfinex.com/ws/2")
	err := client.Connect()
	if err != nil {
		log.Fatalf("connecting authenticated websocket: %s", err)
	}
	defer client.Close()

	ctx, cancelCtx := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelCtx()
	subFundingBook(ctx, client, bitfinex.FundingPrefix+"UST")

	for obj := range client.Listen() {
		switch obj.(type) {
		case error:
			log.Fatalf("channel closed: %s", obj)
			break
		case *websocket.AuthEvent:
		default:
			log.Printf("MSG RECV: %#v", obj)
		}
	}

	time.Sleep(time.Second * 10)
}
