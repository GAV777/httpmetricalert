package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/GAV777/httpmetricalert/internal/agent"
	"github.com/GAV777/httpmetricalert/internal/config"
	grpcclient "github.com/GAV777/httpmetricalert/internal/grpcclient"
	"github.com/GAV777/httpmetricalert/internal/model"
	"github.com/GAV777/httpmetricalert/pkg/crypto"
	"github.com/GAV777/httpmetricalert/pkg/hash"
	"github.com/GAV777/httpmetricalert/pkg/netutil"
	"github.com/GAV777/httpmetricalert/pkg/retry"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

func init() {
	// Флаг -config/-c регистрируется в config.init()
}

// runtimeCollector собирает метрики runtime
func runtimeCollector(store *agent.Store, interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			store.SetGauge("Alloc", float64(m.Alloc))
			store.SetGauge("BuckHashSys", float64(m.BuckHashSys))
			store.SetGauge("Frees", float64(m.Frees))
			store.SetGauge("GCCPUFraction", m.GCCPUFraction)
			store.SetGauge("GCSys", float64(m.GCSys))
			store.SetGauge("HeapAlloc", float64(m.HeapAlloc))
			store.SetGauge("HeapIdle", float64(m.HeapIdle))
			store.SetGauge("HeapInuse", float64(m.HeapInuse))
			store.SetGauge("HeapObjects", float64(m.HeapObjects))
			store.SetGauge("HeapReleased", float64(m.HeapReleased))
			store.SetGauge("HeapSys", float64(m.HeapSys))
			store.SetGauge("LastGC", float64(m.LastGC))
			store.SetGauge("Lookups", float64(m.Lookups))
			store.SetGauge("MCacheInuse", float64(m.MCacheInuse))
			store.SetGauge("MCacheSys", float64(m.MCacheSys))
			store.SetGauge("MSpanInuse", float64(m.MSpanInuse))
			store.SetGauge("MSpanSys", float64(m.MSpanSys))
			store.SetGauge("Mallocs", float64(m.Mallocs))
			store.SetGauge("NextGC", float64(m.NextGC))
			store.SetGauge("NumForcedGC", float64(m.NumForcedGC))
			store.SetGauge("NumGC", float64(m.NumGC))
			store.SetGauge("OtherSys", float64(m.OtherSys))
			store.SetGauge("PauseTotalNs", float64(m.PauseTotalNs))
			store.SetGauge("StackInuse", float64(m.StackInuse))
			store.SetGauge("StackSys", float64(m.StackSys))
			store.SetGauge("Sys", float64(m.Sys))
			store.SetGauge("TotalAlloc", float64(m.TotalAlloc))

			store.SetGauge("RandomValue", rand.Float64())
			store.IncrCounter("PollCount", 1)
		}
	}
}

// gopsutilCollector собирает системные метрики
func gopsutilCollector(store *agent.Store, interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			// Память
			v, err := mem.VirtualMemory()
			if err == nil {
				store.SetGauge("TotalMemory", float64(v.Total))
				store.SetGauge("FreeMemory", float64(v.Free))
			}

			// CPU utilization по ядрам
			percentages, err := cpu.Percent(0, true)
			if err == nil {
				for i, p := range percentages {
					name := fmt.Sprintf("CPUutilization%d", i+1)
					store.SetGauge(name, p)
				}
			}
		}
	}
}

// requestPayload описывает, что отправлять и куда
type requestPayload struct {
	data      []byte // JSON до шифрования/сжатия (для HMAC)
	body      []byte // Готовое тело запроса
	url       string
	useCrypto bool
	secretKey string // Для HMAC-подписи
	errLabel  string // "metric %s" или "batch" для ошибок
}

// buildRequest подготавливает тело запроса: marshal → encrypt/gzip
func buildRequest(jsonData []byte, pubKey *rsa.PublicKey, url, secretKey, errLabel string) (requestPayload, error) {
	payload := requestPayload{
		data:      jsonData,
		url:       url,
		useCrypto: pubKey != nil,
		secretKey: secretKey,
		errLabel:  errLabel,
	}

	if pubKey != nil {
		encrypted, err := crypto.Encrypt(pubKey, jsonData)
		if err != nil {
			return payload, fmt.Errorf("encrypt %s: %w", errLabel, err)
		}
		payload.body = encrypted
	} else {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(jsonData); err != nil {
			gz.Close()
			return payload, fmt.Errorf("compress %s: %w", errLabel, err)
		}
		gz.Close()
		payload.body = buf.Bytes()
	}

	return payload, nil
}

// doRequest выполняет HTTP-запрос с retry, заголовками и HMAC
func doRequest(client *http.Client, payload requestPayload, agentIP string) error {
	cfg := retry.DefaultConfig()
	return retry.Do(context.Background(), cfg, func() error {
		req, err := http.NewRequest("POST", payload.url, bytes.NewReader(payload.body))
		if err != nil {
			return err
		}

		if payload.useCrypto {
			req.Header.Set("Content-Type", "application/octet-stream")
			req.Header.Set("X-Crypto", "rsa")
		} else {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Content-Encoding", "gzip")
		}
		req.Header.Set("Accept-Encoding", "gzip")

		if agentIP != "" {
			req.Header.Set("X-Real-IP", agentIP)
		}

		if key := payload.secretKey; key != "" && !payload.useCrypto {
			h := hash.Sign(string(payload.data), key)
			req.Header.Set("HashSHA256", h)
		}

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("server %d: %s", resp.StatusCode, string(body))
		}
		return nil
	})
}

// sendMetric отправляет одну метрику
func sendMetric(client *http.Client, baseURL string, metric model.Metrics, pubKey *rsa.PublicKey, agentIP, secretKey string) error {
	data, err := json.Marshal(metric)
	if err != nil {
		return fmt.Errorf("marshal metric %s: %w", metric.ID, err)
	}

	payload, err := buildRequest(data, pubKey, fmt.Sprintf("%s/update", baseURL), secretKey, fmt.Sprintf("metric %s", metric.ID))
	if err != nil {
		return err
	}

	return doRequest(client, payload, agentIP)
}

// sendBatch отправляет батч метрик
func sendBatch(client *http.Client, baseURL string, batch []model.Metrics, pubKey *rsa.PublicKey, agentIP, secretKey string) error {
	if len(batch) == 0 {
		return nil
	}

	data, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("marshal batch: %w", err)
	}

	payload, err := buildRequest(data, pubKey, fmt.Sprintf("%s/updates/", baseURL), secretKey, "batch")
	if err != nil {
		return err
	}

	return doRequest(client, payload, agentIP)
}

// senderGRPC — отправляет метрики через gRPC каждые reportInterval
func senderGRPC(grpcClient *grpcclient.Client, store *agent.Store, interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			gauges, counters := store.Snapshot()

			var batch []model.Metrics
			batch = append(batch, gauges...)
			batch = append(batch, counters...)

			if len(batch) == 0 {
				continue
			}

			fmt.Printf("Sending batch via gRPC with %d metrics\n", len(batch))
			ctx := context.Background()
			if err := grpcClient.SendBatch(ctx, batch); err != nil {
				fmt.Printf("Failed to send batch via gRPC: %v\n", err)
			} else {
				fmt.Printf("Successfully sent batch via gRPC with %d metrics\n", len(batch))
			}
		}
	}
}

// sendFinalMetricsGRPC отправляет финальный снимок метрик через gRPC
func sendFinalMetricsGRPC(grpcClient *grpcclient.Client, store *agent.Store) {
	gauges, counters := store.Snapshot()

	var batch []model.Metrics
	batch = append(batch, gauges...)
	batch = append(batch, counters...)

	if len(batch) == 0 {
		fmt.Println("No metrics to send on shutdown")
		return
	}

	fmt.Printf("Sending final batch via gRPC with %d metrics\n", len(batch))
	ctx := context.Background()
	if err := grpcClient.SendBatch(ctx, batch); err != nil {
		fmt.Printf("Failed to send final batch via gRPC: %v\n", err)
	} else {
		fmt.Printf("Successfully sent final batch via gRPC with %d metrics\n", len(batch))
	}
}

// sender — отправляет метрики каждые reportInterval
func sender(store *agent.Store, wp *workerPool, interval time.Duration, stopCh <-chan struct{}, pubKey *rsa.PublicKey) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			gauges, counters := store.Snapshot()

			// Отправляем батч
			var batch []model.Metrics
			batch = append(batch, gauges...)
			batch = append(batch, counters...)

			if len(batch) == 0 {
				continue
			}

			fmt.Printf("Sending batch with %d metrics\n", len(batch))
			wp.Submit(func() {
				if err := sendBatch(wp.client, wp.baseURL, batch, pubKey, wp.agentIP, wp.secretKey); err != nil {
					fmt.Printf("Failed to send batch: %v\n", err)
				} else {
					fmt.Printf("Successfully sent batch with %d metrics\n", len(batch))
				}
			})
		}
	}
}

// sendFinalMetrics отправляет финальный снимок метрик перед остановкой
func sendFinalMetrics(store *agent.Store, wp *workerPool, pubKey *rsa.PublicKey) {
	gauges, counters := store.Snapshot()

	var batch []model.Metrics
	batch = append(batch, gauges...)
	batch = append(batch, counters...)

	if len(batch) == 0 {
		fmt.Println("No metrics to send on shutdown")
		return
	}

	fmt.Printf("Sending final batch with %d metrics\n", len(batch))
	if err := sendBatch(wp.client, wp.baseURL, batch, pubKey, wp.agentIP, wp.secretKey); err != nil {
		fmt.Printf("Failed to send final batch: %v\n", err)
	} else {
		fmt.Printf("Successfully sent final batch with %d metrics\n", len(batch))
	}
}

func main() {
	printBuildInfo()

	cfg, err := config.NewLoader(nil).Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	serverAddress := cfg.ServerAddress()
	reportDuration := time.Duration(cfg.ReportInterval) * time.Second
	pollDuration := time.Duration(cfg.PollInterval) * time.Second
	rateLimit := cfg.RateLimit
	cryptoKeyPath := cfg.CryptoKey
	useGRPC := cfg.UseGRPC
	grpcAddress := cfg.GRPCAddress

	if !strings.HasPrefix(serverAddress, "http://") && !strings.HasPrefix(serverAddress, "https://") {
		serverAddress = "http://" + serverAddress
	}

	fmt.Printf("Starting agent with server address: %s\n", serverAddress)
	fmt.Printf("Report interval: %v, Poll interval: %v, Rate limit: %d\n", reportDuration, pollDuration, rateLimit)
	fmt.Printf("Use gRPC: %v, gRPC address: %s\n", useGRPC, grpcAddress)

	// Загружаем публичный ключ, если указан
	var pubKey *rsa.PublicKey
	if cryptoKeyPath != "" {
		var err error
		pubKey, err = crypto.LoadPublicKey(cryptoKeyPath)
		if err != nil {
			log.Fatalf("Failed to load public key: %v", err)
		}
		fmt.Println("RSA encryption enabled")
	}

	store := agent.NewStore()
	agentIP := getLocalIP()

	// Запускаем сборщики метрик в отдельных горутинах
	stopCollect := make(chan struct{})
	go runtimeCollector(store, pollDuration, stopCollect)
	go gopsutilCollector(store, pollDuration, stopCollect)

	if useGRPC {
		runGRPCAgent(grpcAddress, agentIP, store, reportDuration, stopCollect)
	} else {
		runHTTPAgent(serverAddress, agentIP, store, reportDuration, stopCollect, pubKey, rateLimit, cfg.Key)
	}
}

// runGRPCAgent запускает агент с gRPC-транспортом
func runGRPCAgent(grpcAddress, agentIP string, store *agent.Store, reportDuration time.Duration, stopCollect chan struct{}) {
	if grpcAddress == "" {
		log.Fatal("gRPC address is required when USE_GRPC is enabled")
	}

	grpcClient, err := grpcclient.NewClient(grpcAddress, agentIP)
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer grpcClient.Close()

	fmt.Println("Connected to gRPC server")

	// Запускаем gRPC-отправителя
	go senderGRPC(grpcClient, store, reportDuration, stopCollect)

	// Ждём сигнал завершения
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	<-sigCh

	fmt.Println("\nShutting down agent...")

	// Останавливаем сборщики
	close(stopCollect)

	// Даём горутинам завершиться
	time.Sleep(100 * time.Millisecond)

	// Отправляем финальный снимок метрик через gRPC
	sendFinalMetricsGRPC(grpcClient, store)

	fmt.Println("Agent stopped gracefully")
}

// runHTTPAgent запускает агент с HTTP-транспортом
func runHTTPAgent(serverAddress, agentIP string, store *agent.Store, reportDuration time.Duration, stopCollect chan struct{}, pubKey *rsa.PublicKey, rateLimit int, secretKey string) {
	client := &http.Client{}

	// Запускаем worker pool
	wp := newWorkerPool(rateLimit, client, serverAddress, agentIP, secretKey)
	wp.Start()

	// Запускаем отправителя
	go sender(store, wp, reportDuration, stopCollect, pubKey)

	// Ждём сигнал завершения
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	<-sigCh

	fmt.Println("\nShutting down agent...")

	// Останавливаем сборщики
	close(stopCollect)

	// Даём горутинам завершиться
	time.Sleep(100 * time.Millisecond)

	// Отправляем финальный снимок метрик
	sendFinalMetrics(store, wp, pubKey)

	// Останавливаем worker pool
	wp.Stop()

	fmt.Println("Agent stopped gracefully")
}

// getLocalIP возвращает IP-адрес хоста
func getLocalIP() string {
	return netutil.LocalIP()
}

func printBuildInfo() {
	fmt.Printf("Build version: %s\n", buildVersion)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Build commit: %s\n", buildCommit)
}
