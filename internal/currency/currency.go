package currency

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type CurrencyMeta struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
	Flag   string `json:"flag"`
	IsCrypto bool `json:"is_crypto"`
}

type CryptoPrice struct {
	ID        string  `json:"id"`
	Symbol    string  `json:"symbol"`
	Name      string  `json:"name"`
	PriceUSD  float64 `json:"price_usd"`
	PriceIDR  float64 `json:"price_idr"`
	Change24h float64 `json:"change_24h"`
	Icon      string  `json:"icon"`
}

type ConversionResult struct {
	FromCode    string    `json:"from_code"`
	ToCode      string    `json:"to_code"`
	Amount      float64   `json:"amount"`
	Result      float64   `json:"result"`
	Rate        float64   `json:"rate"`
	InverseRate float64   `json:"inverse_rate"`
	Formatted   string    `json:"formatted"`
	Timestamp   time.Time `json:"timestamp"`
}

type RatesData struct {
	Base        string             `json:"base"`
	Rates       map[string]float64 `json:"rates"`
	LastUpdated time.Time          `json:"last_updated"`
}

type Service struct {
	client      *http.Client
	mu          sync.RWMutex
	ratesToUSD  map[string]float64
	lastUpdated time.Time
	cryptoRates map[string]CryptoPrice
}

var CurrencyList = []CurrencyMeta{
	{Code: "USD", Name: "US Dollar", Symbol: "$", Flag: "🇺🇸"},
	{Code: "IDR", Name: "Indonesian Rupiah", Symbol: "Rp", Flag: "🇮🇩"},
	{Code: "EUR", Name: "Euro", Symbol: "€", Flag: "🇪🇺"},
	{Code: "GBP", Name: "British Pound", Symbol: "£", Flag: "🇬🇧"},
	{Code: "JPY", Name: "Japanese Yen", Symbol: "¥", Flag: "🇯🇵"},
	{Code: "SGD", Name: "Singapore Dollar", Symbol: "S$", Flag: "🇸🇬"},
	{Code: "MYR", Name: "Malaysian Ringgit", Symbol: "RM", Flag: "🇲🇾"},
	{Code: "AUD", Name: "Australian Dollar", Symbol: "A$", Flag: "🇦🇺"},
	{Code: "CAD", Name: "Canadian Dollar", Symbol: "C$", Flag: "🇨🇦"},
	{Code: "CHF", Name: "Swiss Franc", Symbol: "CHF", Flag: "🇨🇭"},
	{Code: "CNY", Name: "Chinese Yuan", Symbol: "¥", Flag: "🇨🇳"},
	{Code: "HKD", Name: "Hong Kong Dollar", Symbol: "HK$", Flag: "🇭🇰"},
	{Code: "KRW", Name: "South Korean Won", Symbol: "₩", Flag: "🇰🇷"},
	{Code: "INR", Name: "Indian Rupee", Symbol: "₹", Flag: "🇮🇳"},
	{Code: "THB", Name: "Thai Baht", Symbol: "฿", Flag: "🇹🇭"},
	{Code: "PHP", Name: "Philippine Peso", Symbol: "₱", Flag: "🇵🇭"},
	{Code: "VND", Name: "Vietnamese Dong", Symbol: "₫", Flag: "🇻🇳"},
	{Code: "SAR", Name: "Saudi Riyal", Symbol: "﷼", Flag: "🇸🇦"},
	{Code: "AED", Name: "UAE Dirham", Symbol: "د.إ", Flag: "🇦🇪"},
	{Code: "TRY", Name: "Turkish Lira", Symbol: "₺", Flag: "🇹🇷"},
	{Code: "BRL", Name: "Brazilian Real", Symbol: "R$", Flag: "🇧🇷"},
	{Code: "MXN", Name: "Mexican Peso", Symbol: "$", Flag: "🇲🇽"},
	{Code: "NZD", Name: "New Zealand Dollar", Symbol: "NZ$", Flag: "🇳🇿"},
	{Code: "RUB", Name: "Russian Ruble", Symbol: "₽", Flag: "🇷🇺"},
	{Code: "ZAR", Name: "South African Rand", Symbol: "R", Flag: "🇿🇦"},
	{Code: "SEK", Name: "Swedish Krona", Symbol: "kr", Flag: "🇸🇪"},
	{Code: "NOK", Name: "Norwegian Krone", Symbol: "kr", Flag: "🇳🇴"},
	{Code: "DKK", Name: "Danish Krone", Symbol: "kr", Flag: "🇩🇰"},
	{Code: "PLN", Name: "Polish Zloty", Symbol: "zł", Flag: "🇵🇱"},
	{Code: "TWD", Name: "New Taiwan Dollar", Symbol: "NT$", Flag: "🇹🇼"},

	// Top Cryptos
	{Code: "BTC", Name: "Bitcoin", Symbol: "₿", Flag: "🪙", IsCrypto: true},
	{Code: "ETH", Name: "Ethereum", Symbol: "Ξ", Flag: "🪙", IsCrypto: true},
	{Code: "SOL", Name: "Solana", Symbol: "SOL", Flag: "🪙", IsCrypto: true},
	{Code: "BNB", Name: "BNB", Symbol: "BNB", Flag: "🪙", IsCrypto: true},
	{Code: "XRP", Name: "Ripple", Symbol: "XRP", Flag: "🪙", IsCrypto: true},
	{Code: "DOGE", Name: "Dogecoin", Symbol: "Ð", Flag: "🪙", IsCrypto: true},
	{Code: "ADA", Name: "Cardano", Symbol: "ADA", Flag: "🪙", IsCrypto: true},
	{Code: "USDT", Name: "Tether USD", Symbol: "₮", Flag: "🪙", IsCrypto: true},
}

func NewService() *Service {
	// Baseline fallback rates relative to 1 USD
	fallback := map[string]float64{
		"USD": 1.0,
		"IDR": 16250.0,
		"EUR": 0.92,
		"GBP": 0.79,
		"JPY": 155.0,
		"SGD": 1.35,
		"MYR": 4.71,
		"AUD": 1.52,
		"CAD": 1.37,
		"CHF": 0.91,
		"CNY": 7.24,
		"HKD": 7.81,
		"KRW": 1380.0,
		"INR": 83.5,
		"THB": 36.8,
		"PHP": 58.5,
		"VND": 25400.0,
		"SAR": 3.75,
		"AED": 3.67,
		"TRY": 32.5,
		"BRL": 5.35,
		"MXN": 18.2,
		"NZD": 1.63,
		"RUB": 89.0,
		"ZAR": 18.6,
		"SEK": 10.6,
		"NOK": 10.7,
		"DKK": 6.87,
		"PLN": 3.97,
		"TWD": 32.4,

		// Default crypto USD multipliers (stored as units per 1 USD)
		"BTC": 0.0000154, // ~65,000 USD
		"ETH": 0.000286,  // ~3,500 USD
		"SOL": 0.0069,    // ~145 USD
		"BNB": 0.0017,    // ~580 USD
		"XRP": 1.85,      // ~0.54 USD
		"DOGE": 8.0,      // ~0.125 USD
		"ADA": 2.5,       // ~0.40 USD
		"USDT": 1.0,
	}

	s := &Service{
		client:      &http.Client{Timeout: 6 * time.Second},
		ratesToUSD:  fallback,
		lastUpdated: time.Now(),
		cryptoRates: make(map[string]CryptoPrice),
	}

	// Fetch fresh rates in background immediately
	go s.RefreshRates(context.Background())

	// Refresh periodic ticker every 30 minutes
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		for range ticker.C {
			s.RefreshRates(context.Background())
		}
	}()

	return s
}

// RefreshRates updates fiat rates from open.er-api.com and crypto from CoinGecko
func (s *Service) RefreshRates(ctx context.Context) {
	// 1. Fetch Fiat Rates
	req, err := http.NewRequestWithContext(ctx, "GET", "https://open.er-api.com/v6/latest/USD", nil)
	if err == nil {
		req.Header.Set("User-Agent", "SearXGo-Currency/1.0")
		resp, err := s.client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var data struct {
				Rates map[string]float64 `json:"rates"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&data); err == nil && len(data.Rates) > 0 {
				s.mu.Lock()
				for k, v := range data.Rates {
					s.ratesToUSD[strings.ToUpper(k)] = v
				}
				s.lastUpdated = time.Now()
				s.mu.Unlock()
			}
		}
	}

	// 2. Fetch Crypto Rates (CoinGecko public simple price)
	cgURL := "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin,ethereum,binancecoin,solana,ripple,dogecoin,cardano,tether&vs_currencies=usd,idr&include_24hr_change=true"
	cReq, err := http.NewRequestWithContext(ctx, "GET", cgURL, nil)
	if err == nil {
		cReq.Header.Set("User-Agent", "SearXGo-Currency/1.0")
		cResp, err := s.client.Do(cReq)
		if err == nil && cResp.StatusCode == http.StatusOK {
			defer cResp.Body.Close()
			var cgData map[string]struct {
				USD       float64 `json:"usd"`
				IDR       float64 `json:"idr"`
				USDChange float64 `json:"usd_24h_change"`
			}
			if err := json.NewDecoder(cResp.Body).Decode(&cgData); err == nil {
				idToSymbol := map[string]struct {
					Sym  string
					Name string
					Icon string
				}{
					"bitcoin":     {"BTC", "Bitcoin", "🪙"},
					"ethereum":    {"ETH", "Ethereum", "Ξ"},
					"solana":      {"SOL", "Solana", "☀️"},
					"binancecoin": {"BNB", "BNB", "🟡"},
					"ripple":      {"XRP", "Ripple", "💧"},
					"dogecoin":    {"DOGE", "Dogecoin", "🐕"},
					"cardano":     {"ADA", "Cardano", "🔷"},
					"tether":      {"USDT", "Tether", "₮"},
				}

				s.mu.Lock()
				for coinID, val := range cgData {
					info, ok := idToSymbol[coinID]
					if !ok {
						continue
					}
					if val.USD > 0 {
						// Store units per 1 USD
						s.ratesToUSD[info.Sym] = 1.0 / val.USD
					}
					s.cryptoRates[info.Sym] = CryptoPrice{
						ID:        coinID,
						Symbol:    info.Sym,
						Name:      info.Name,
						PriceUSD:  val.USD,
						PriceIDR:  val.IDR,
						Change24h: val.USDChange,
						Icon:      info.Icon,
					}
				}
				s.mu.Unlock()
			}
		}
	}
}

// Convert converts an amount between two currencies
func (s *Service) Convert(from, to string, amount float64) (*ConversionResult, error) {
	from = strings.ToUpper(strings.TrimSpace(from))
	to = strings.ToUpper(strings.TrimSpace(to))

	if amount < 0 {
		amount = 0
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	fromRate, ok1 := s.ratesToUSD[from]
	toRate, ok2 := s.ratesToUSD[to]

	if !ok1 {
		return nil, fmt.Errorf("currency '%s' is not supported", from)
	}
	if !ok2 {
		return nil, fmt.Errorf("currency '%s' is not supported", to)
	}

	// Rate relative to USD: fromRate = how many 'FROM' per 1 USD
	// Therefore 1 FROM = (1 / fromRate) USD
	// And (1 / fromRate) * toRate = how many 'TO' per 1 FROM
	unitRate := (1.0 / fromRate) * toRate
	res := amount * unitRate
	inverse := 0.0
	if unitRate > 0 {
		inverse = 1.0 / unitRate
	}

	formatted := fmt.Sprintf("%.2f", res)
	if res >= 1000 || to == "IDR" || to == "VND" || to == "KRW" || to == "JPY" {
		formatted = formatCurrencyNumber(res, 2)
	} else if res < 0.001 && res > 0 {
		formatted = fmt.Sprintf("%.8f", res)
	}

	return &ConversionResult{
		FromCode:    from,
		ToCode:      to,
		Amount:      amount,
		Result:      res,
		Rate:        unitRate,
		InverseRate: inverse,
		Formatted:   formatted,
		Timestamp:   s.lastUpdated,
	}, nil
}

// GetRatesSnapshot returns map of all current rates relative to 1 USD
func (s *Service) GetRatesSnapshot() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make(map[string]float64, len(s.ratesToUSD))
	for k, v := range s.ratesToUSD {
		res[k] = v
	}
	return res
}

// GetTopCryptos returns the list of cached top cryptocurrencies
func (s *Service) GetTopCryptos() []CryptoPrice {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []CryptoPrice
	for _, c := range s.cryptoRates {
		list = append(list, c)
	}
	// Sort by symbol or popularity
	order := map[string]int{
		"BTC": 1, "ETH": 2, "SOL": 3, "BNB": 4, "XRP": 5, "DOGE": 6, "ADA": 7, "USDT": 8,
	}
	sort.Slice(list, func(i, j int) bool {
		oi := order[list[i].Symbol]
		oj := order[list[j].Symbol]
		if oi != 0 && oj != 0 {
			return oi < oj
		}
		return list[i].Symbol < list[j].Symbol
	})
	return list
}

func formatCurrencyNumber(val float64, decimals int) string {
	parts := strings.Split(fmt.Sprintf("%.*f", decimals, val), ".")
	intPart := parts[0]
	decPart := ""
	if len(parts) > 1 {
		decPart = parts[1]
	}

	var res []string
	length := len(intPart)
	for i := length; i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		res = append([]string{intPart[start:i]}, res...)
	}

	commaInt := strings.Join(res, ",")
	if decPart != "" && decPart != "00" {
		return commaInt + "." + decPart
	}
	return commaInt
}
