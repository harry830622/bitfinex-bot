package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
	"fmt"

	bfx "github.com/bitfinexcom/bitfinex-api-go/v2"
	bfxWs "github.com/bitfinexcom/bitfinex-api-go/v2/websocket"

	ws "github.com/gorilla/websocket"
)

var upgrader = ws.Upgrader{CheckOrigin: func(r *http.Request) bool {
	return true // TODO: Remove when production
},
}

var amtByRateBySideBySymbol = make(map[string](map[bfx.OrderSide](map[float64]float64)))
var symbols = []string{bfx.FundingPrefix + "USD", bfx.FundingPrefix + "UST"}

func createClient(url string) *bfxWs.Client {
	key := os.Getenv("BFX_API_KEY")
	secret := os.Getenv("BFX_API_SECRET")
	param := bfxWs.NewDefaultParameters()
	param.URL = url
	client := bfxWs.NewWithParams(param).Credentials(key, secret)
	return client
}

func subBook(ctx context.Context, client *bfxWs.Client, symbol string) {
	_, err := client.SubscribeBook(ctx, symbol, bfx.Precision0, bfx.FrequencyRealtime, 25)
	if err != nil {
		log.Fatalf("subscribing to book: %s", err)
	}
}

func listenBfxWs(client *bfxWs.Client, ch chan interface{}) {
	for obj := range client.Listen() {
		ch <- obj
	}
	close(ch)
}

func wsHandler(res http.ResponseWriter, req *http.Request) {
	conn, err := upgrader.Upgrade(res, req, nil)
	if err != nil {
		log.Print("Failed to upgrade:", err)
		return
	}
	defer conn.Close()
	for {
		amtByRateBySideBySymbolJson := make(map[string](map[string](map[string]float64)))
		for symbol, amtByRateBySide := range amtByRateBySideBySymbol {
			amtByRateBySideBySymbolJson[symbol] = make(map[string](map[string]float64))
			for side, amtByRate := range amtByRateBySide {
				sideStr := fmt.Sprintf("%d", side)
				amtByRateBySideBySymbolJson[symbol][sideStr] = make(map[string]float64)
				for rate, amt := range amtByRate {
					rateStr := fmt.Sprintf("%f", rate)
					amtByRateBySideBySymbolJson[symbol][sideStr][rateStr] = amt
				}
			}
		}
		json, err := json.Marshal(amtByRateBySideBySymbolJson)
		err = conn.WriteMessage(ws.TextMessage, json)
		if err != nil {
			log.Println("Failed to write the message:", err)
			break
		}
		time.Sleep(1 * time.Second)
	}
}

func main() {
	client := createClient("wss://api.bitfinex.com/ws/2")
	err := client.Connect()
	if err != nil {
		log.Fatalf("connecting authenticated websocket: %s", err)
	}
	defer client.Close()

	ch := make(chan interface{})
	go listenBfxWs(client, ch)

	for _, symbol := range symbols {
		amtByRateBySideBySymbol[symbol] = make(map[bfx.OrderSide](map[float64]float64))
		for _, side := range []bfx.OrderSide{bfx.Bid, bfx.Ask} {
			amtByRateBySideBySymbol[symbol][side] = make(map[float64]float64)
		}
	}
	for _, symbol := range symbols {
		ctx, cancelCtx := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelCtx()
		subBook(ctx, client, symbol)
	}

	http.HandleFunc("/ws", wsHandler)
	go http.ListenAndServe(":5000", nil)

	for {
		obj := <-ch
		switch obj.(type) {
		case error:
			log.Fatalf("channel closed: %s", obj)
			break
		case *bfx.BookUpdateSnapshot:
			bookUpdateSnapshot := obj.(*bfx.BookUpdateSnapshot)
			for _, snapshot := range bookUpdateSnapshot.Snapshot {
				amtByRateBySideBySymbol[snapshot.Symbol][snapshot.Side][snapshot.Rate] += snapshot.Amount
			}
		case *bfx.BookUpdate:
			bookUpdate := obj.(*bfx.BookUpdate)
			if bookUpdate.Action == bfx.BookUpdateEntry {
				amtByRateBySideBySymbol[bookUpdate.Symbol][bookUpdate.Side][bookUpdate.Rate] += bookUpdate.Amount
			} else {
				delete(amtByRateBySideBySymbol[bookUpdate.Symbol][bookUpdate.Side], bookUpdate.Rate)
			}
			log.Println(amtByRateBySideBySymbol)
		default:
			log.Printf("MSG RECV: %#v", obj)
		}
	}
}
