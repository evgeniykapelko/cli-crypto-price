package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"log"
	"net/http"
	"sync"
	"time"
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
	Price    float64
	Source   string
	Duration time.Duration
}

func fetchCryptoPriceFromCoingecko(crypto string, ch chan<- PriceResult, wg *sync.WaitGroup) {
	defer wg.Done()
	url := fmt.Sprintf(coingeckoAPI, crypto)
	start := time.Now()
	resp, err := http.Get(url)
	duration := time.Since(start)
	if err != nil {
		ch <- PriceResult{0, "CoinGecko", duration}
		return
	}
	defer resp.Body.Close()

	var result map[string]CryptoPrice
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		ch <- PriceResult{0, "CoinGecko", duration}
		return
	}

	price, ok := result[crypto]
	if ok {
		ch <- PriceResult{price.USD, "CoinGecko", duration}
	} else {
		ch <- PriceResult{0, "CoinGecko", duration}
	}
}

func fetchCryptoPriceFromCoinMarketCap(crypto string, ch chan<- PriceResult, wg *sync.WaitGroup) {
	defer wg.Done()
	url := fmt.Sprintf(coinmarketcapAPI, crypto)
	start := time.Now()
	resp, err := http.Get(url)
	duration := time.Since(start)
	if err != nil {
		ch <- PriceResult{0, "CoinMarketCap", duration}
		return
	}
	defer resp.Body.Close()

	var result []CoinMarketCapResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		ch <- PriceResult{0, "CoinMarketCap", duration}
		return
	}

	if len(result) > 0 {
		var price float64
		fmt.Sscanf(result[0].PriceUSD, "%f", &price)
		ch <- PriceResult{price, "CoinMarketCap", duration}
	} else {
		ch <- PriceResult{0, "CoinMarketCap", duration}
	}
}

func fetchCryptoPriceFromCryptoCompare(crypto string, ch chan<- PriceResult, wg *sync.WaitGroup) {
	defer wg.Done()
	url := fmt.Sprintf(cryptocompareAPI, crypto)
	start := time.Now()
	resp, err := http.Get(url)
	duration := time.Since(start)
	if err != nil {
		ch <- PriceResult{0, "CryptoCompare", duration}
		return
	}
	defer resp.Body.Close()

	var result CryptoCompareResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		ch <- PriceResult{0, "CryptoCompare", duration}
		return
	}

	ch <- PriceResult{result.USD, "CryptoCompare", duration}
}

func fetchCryptoPriceConcurrently(crypto string) PriceResult {
	ch := make(chan PriceResult, 3)
	var wg sync.WaitGroup

	wg.Add(3)
	go fetchCryptoPriceFromCoingecko(crypto, ch, &wg)
	go fetchCryptoPriceFromCoinMarketCap(crypto, ch, &wg)
	go fetchCryptoPriceFromCryptoCompare(crypto, ch, &wg)

	go func() {
		wg.Wait()
		close(ch)
	}()

	for result := range ch {
		if result.Price > 0 {
			return result
		}
	}

	return PriceResult{0, "None", 0}
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
			fmt.Printf("The current price of %s is $%.2f (Source: %s, Duration: %s)\n", crypto, result.Price, result.Source, result.Duration)
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
