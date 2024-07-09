package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

const (
	coingeckoAPI     = "https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd"
	coinmarketcapAPI = "https://api.coinmarketcap.com/v1/ticker/%s/"
	cryptocompareAPI = "https://min-api.cryptocompare.com/data/price?fsym=%s&tsyms=USD"
)

type CryptoPrice struct {
	USD float64 `json:"usd"`
}

type CoinMarketCapResponse struct {
	PriceUSD string `json:"price_usd"`
}

type CryptoCompareResponse struct {
	USD float64 `json:"USD"`
}

type PriceResult struct {
	Price  float64
	Source string
}

func fetchCryptoPrice(ctx context.Context, url string, parseFunc func([]byte) (float64, error), source string, ch chan<- PriceResult, wg *sync.WaitGroup) {
	defer wg.Done()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		ch <- PriceResult{0, source}
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		ch <- PriceResult{0, source}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ch <- PriceResult{0, source}
		return
	}

	body := json.NewDecoder(resp.Body).Decode
	data, err := json.Marshal(body)
	if err != nil {
		ch <- PriceResult{0, source}
		return
	}

	price, err := parseFunc(data)
	if err != nil {
		ch <- PriceResult{0, source}
		return
	}

	ch <- PriceResult{price, source}
}

func parseCoingecko(data []byte) (float64, error) {
	var result map[string]CryptoPrice
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, err
	}
	for _, v := range result {
		return v.USD, nil
	}
	return 0, fmt.Errorf("invalid response")
}

func parseCoinMarketCap(data []byte) (float64, error) {
	var result []CoinMarketCapResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, err
	}
	if len(result) > 0 {
		var price float64
		fmt.Sscanf(result[0].PriceUSD, "%f", &price)
		return price, nil
	}
	return 0, fmt.Errorf("invalid response")
}

func parseCryptoCompare(data []byte) (float64, error) {
	var result CryptoCompareResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, err
	}
	return result.USD, nil
}

func fetchCryptoPriceConcurrently(crypto string) PriceResult {
	ch := make(chan PriceResult, 3)
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(3)
	go fetchCryptoPrice(ctx, fmt.Sprintf(coingeckoAPI, crypto), parseCoingecko, "CoinGecko", ch, &wg)
	go fetchCryptoPrice(ctx, fmt.Sprintf(coinmarketcapAPI, crypto), parseCoinMarketCap, "CoinMarketCap", ch, &wg)
	go fetchCryptoPrice(ctx, fmt.Sprintf(cryptocompareAPI, crypto), parseCryptoCompare, "CryptoCompare", ch, &wg)

	var result PriceResult

	select {
	case result = <-ch:
		cancel()
	case <-time.After(10 * time.Second):
		cancel()
	}

	wg.Wait()
	close(ch)

	for priceResult := range ch {
		if priceResult.Price > 0 {
			result = priceResult
			break
		}
	}

	return result
}

var rootCmd = &cobra.Command{
	Use:   "crypto-cli",
	Short: "A CLI tool to fetch cryptocurrency prices",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("Please specify a cryptocurrency (e.g., bitcoin, ethereum)")
			return
		}
		crypto := args[0]
		result := fetchCryptoPriceConcurrently(crypto)
		if result.Price > 0 {
			fmt.Printf("The current price of %s is $%.2f (Source: %s)\n", crypto, result.Price, result.Source)
		} else {
			fmt.Println("Failed to fetch the price")
		}
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
