package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

var (
	resultsDir = pflag.String("results", "results", "Directory containing JSON metric files")
	outputFile = pflag.String("output", "report.md", "Output Markdown report file")
)

type TestResult struct {
	Broker        string
	MessageSize   int
	TargetRate    int
	Sent          int64
	Received      int64
	Errors        int64
	DurationSec   float64
	ActualRate    float64
	AvgLatencyMs  float64
	P95LatencyMs  float64
	MaxLatencyMs  float64
}

func main() {
	pflag.Parse()

	files, err := filepath.Glob(filepath.Join(*resultsDir, "*.json"))
	if err != nil {
		logrus.Fatalf("Failed to list result files: %v", err)
	}

	testMap := make(map[string]*TestResult)

	for _, file := range files {
		base := filepath.Base(file)
		if strings.Contains(base, "producer") {
			if err := processProducerFile(file, testMap); err != nil {
				logrus.WithError(err).Warnf("Skipping file %s", file)
			}
		} else if strings.Contains(base, "consumer") {
			if err := processConsumerFile(file, testMap); err != nil {
				logrus.WithError(err).Warnf("Skipping file %s", file)
			}
		}
	}

	report := generateMarkdown(testMap)

	if err := os.WriteFile(*outputFile, []byte(report), 0644); err != nil {
		logrus.Fatalf("Failed to write report: %v", err)
	}
	logrus.Infof("Report generated: %s", *outputFile)
}

func processProducerFile(path string, testMap map[string]*TestResult) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var prod struct {
		Broker       string  `json:"broker"`
		MessageSize  int     `json:"message_size"`
		TargetRate   int     `json:"target_rate"`
		MessagesSent int64   `json:"messages_sent"`
		Errors       int64   `json:"errors"`
		DurationSec  float64 `json:"duration_sec"`
		ActualRate   float64 `json:"actual_rate"`
	}
	if err := json.Unmarshal(data, &prod); err != nil {
		return err
	}

	key := fmt.Sprintf("%s_%d_%d", prod.Broker, prod.MessageSize, prod.TargetRate)
	if _, ok := testMap[key]; !ok {
		testMap[key] = &TestResult{
			Broker:      prod.Broker,
			MessageSize: prod.MessageSize,
			TargetRate:  prod.TargetRate,
		}
	}
	tr := testMap[key]
	tr.Sent = prod.MessagesSent
	tr.Errors = prod.Errors
	tr.DurationSec = prod.DurationSec
	tr.ActualRate = prod.ActualRate
	return nil
}

func processConsumerFile(path string, testMap map[string]*TestResult) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var cons struct {
		Broker       string  `json:"broker"`
		Received     int64   `json:"messages_received"`
		Errors       int64   `json:"errors"`
		DurationSec  float64 `json:"duration_sec"`
		AvgLatencyMs float64 `json:"avg_latency_ms"`
		P95LatencyMs float64 `json:"p95_latency_ms"`
		MaxLatencyMs float64 `json:"max_latency_ms"`
	}
	if err := json.Unmarshal(data, &cons); err != nil {
		return err
	}

	base := filepath.Base(path)
	parts := strings.Split(base, "_")
	if len(parts) < 3 {
		return fmt.Errorf("unexpected consumer file name: %s", base)
	}
	broker := parts[0]
	sizeStr := strings.TrimPrefix(parts[1], "size")
	rateStr := strings.TrimPrefix(parts[2], "rate")
	size, _ := strconv.Atoi(sizeStr)
	rate, _ := strconv.Atoi(rateStr)

	key := fmt.Sprintf("%s_%d_%d", broker, size, rate)
	if _, ok := testMap[key]; !ok {
		testMap[key] = &TestResult{
			Broker:      broker,
			MessageSize: size,
			TargetRate:  rate,
		}
	}
	tr := testMap[key]
	tr.Received = cons.Received
	tr.AvgLatencyMs = cons.AvgLatencyMs
	tr.P95LatencyMs = cons.P95LatencyMs
	tr.MaxLatencyMs = cons.MaxLatencyMs
	return nil
}

func generateMarkdown(testMap map[string]*TestResult) string {
	var sb strings.Builder

	sb.WriteString("# Отчёт: Сравнение RabbitMQ и Redis как брокеров сообщений\n\n")

	sb.WriteString("## Условия тестирования\n\n")
	sb.WriteString("- Длительность каждого теста: 30 секунд\n")
	sb.WriteString("- Размеры сообщений: 128 B, 1 KB, 10 KB, 100 KB\n")
	sb.WriteString("- Интенсивность: 1000, 5000, 10000 msg/s\n")
	sb.WriteString("- Конфигурация: single instance, настройки по умолчанию\n\n")

	rabbitResults := []*TestResult{}
	redisResults := []*TestResult{}
	for _, tr := range testMap {
		if tr.Broker == "rabbitmq" {
			rabbitResults = append(rabbitResults, tr)
		} else if tr.Broker == "redis" {
			redisResults = append(redisResults, tr)
		}
	}

	sortResults := func(results []*TestResult) {
		sort.Slice(results, func(i, j int) bool {
			if results[i].MessageSize != results[j].MessageSize {
				return results[i].MessageSize < results[j].MessageSize
			}
			return results[i].TargetRate < results[j].TargetRate
		})
	}
	sortResults(rabbitResults)
	sortResults(redisResults)

	sb.WriteString("## Результаты RabbitMQ\n\n")
	sb.WriteString("| Размер (B) | Интенсивность (msg/s) | Отправлено | Получено | Потери | Avg Latency (ms) | p95 Latency (ms) |\n")
	sb.WriteString("|------------|------------------------|------------|----------|--------|------------------|------------------|\n")
	for _, tr := range rabbitResults {
		loss := tr.Sent - tr.Received
		sb.WriteString(fmt.Sprintf("| %d | %d | %d | %d | %d | %.2f | %.2f |\n",
			tr.MessageSize, tr.TargetRate, tr.Sent, tr.Received, loss,
			tr.AvgLatencyMs, tr.P95LatencyMs))
	}

	sb.WriteString("\n## Результаты Redis\n\n")
	sb.WriteString("| Размер (B) | Интенсивность (msg/s) | Отправлено | Получено | Потери | Avg Latency (ms) | p95 Latency (ms) |\n")
	sb.WriteString("|------------|------------------------|------------|----------|--------|------------------|------------------|\n")
	for _, tr := range redisResults {
		loss := tr.Sent - tr.Received
		sb.WriteString(fmt.Sprintf("| %d | %d | %d | %d | %d | %.2f | %.2f |\n",
			tr.MessageSize, tr.TargetRate, tr.Sent, tr.Received, loss,
			tr.AvgLatencyMs, tr.P95LatencyMs))
	}

	sb.WriteString("\n## Анализ и выводы\n\n")
	sb.WriteString(generateConclusions(rabbitResults, redisResults))

	return sb.String()
}

func generateConclusions(rabbit, redis []*TestResult) string {
	var sb strings.Builder

	sb.WriteString("### Пропускная способность\n\n")
	maxRate := 10000
	smallSize := 128
	rabbitMax := getResult(rabbit, smallSize, maxRate)
	redisMax := getResult(redis, smallSize, maxRate)
	if rabbitMax != nil && redisMax != nil {
		sb.WriteString(fmt.Sprintf("- При размере %d B и целевой интенсивности %d msg/s:\n", smallSize, maxRate))
		sb.WriteString(fmt.Sprintf("  - RabbitMQ фактически отправил **%.0f msg/s**\n", rabbitMax.ActualRate))
		sb.WriteString(fmt.Sprintf("  - Redis фактически отправил **%.0f msg/s**\n", redisMax.ActualRate))
		if redisMax.ActualRate > rabbitMax.ActualRate {
			sb.WriteString(fmt.Sprintf("- Redis показал на **%.1f%%** более высокую пропускную способность на малых сообщениях.\n",
				(redisMax.ActualRate-rabbitMax.ActualRate)/rabbitMax.ActualRate*100))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("### Влияние размера сообщения\n\n")
	rateLow := 1000
	for _, size := range []int{128, 1024, 10240, 102400} {
		r := getResult(rabbit, size, rateLow)
		rd := getResult(redis, size, rateLow)
		if r != nil && rd != nil {
			sb.WriteString(fmt.Sprintf("- **%d B**: RabbitMQ p95 = %.2f ms, Redis p95 = %.2f ms. ",
				size, r.P95LatencyMs, rd.P95LatencyMs))
			if rd.P95LatencyMs < r.P95LatencyMs {
				sb.WriteString("Redis имеет меньшую задержку.\n")
			} else {
				sb.WriteString("RabbitMQ имеет меньшую задержку.\n")
			}
		}
	}
	rateHigh := 10000
	r100k := getResult(rabbit, 102400, rateHigh)
	rd100k := getResult(redis, 102400, rateHigh)
	if r100k != nil && rd100k != nil {
		sb.WriteString(fmt.Sprintf("- При нагрузке %d msg/s и размере 100 КБ: ", rateHigh))
		if r100k.P95LatencyMs < rd100k.P95LatencyMs {
			sb.WriteString("RabbitMQ сохраняет приемлемую задержку, в то время как Redis значительно деградирует.\n")
		}
	}
	sb.WriteString("\n")

	sb.WriteString("### Устойчивость к высокой нагрузке\n\n")
	sb.WriteString("Точка деградации определялась по увеличению p95 задержки > 50 мс или потерям > 1%.\n\n")
	for _, brokerResults := range []struct {
		name    string
		results []*TestResult
	}{
		{"RabbitMQ", rabbit},
		{"Redis", redis},
	} {
		sb.WriteString(fmt.Sprintf("**%s**:\n", brokerResults.name))
		for _, size := range []int{128, 1024, 10240, 102400} {
			prevP95 := 0.0
			degradedRate := 0
			for _, rate := range []int{1000, 5000, 10000} {
				tr := getResult(brokerResults.results, size, rate)
				if tr == nil {
					continue
				}
				loss := tr.Sent - tr.Received
				lossPercent := float64(loss) / float64(tr.Sent) * 100
				highLatency := tr.P95LatencyMs > 50 && tr.P95LatencyMs > 0
				if (highLatency && tr.P95LatencyMs > prevP95*2) || lossPercent > 1.0 {
					if degradedRate == 0 {
						degradedRate = rate
					}
				}
				prevP95 = tr.P95LatencyMs
			}
			if degradedRate > 0 {
				sb.WriteString(fmt.Sprintf("- Размер %d B: деградация начинается при **%d msg/s**.\n", size, degradedRate))
			} else {
				sb.WriteString(fmt.Sprintf("- Размер %d B: стабильно работает до 10000 msg/s.\n", size))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("### Рекомендации\n\n")
	sb.WriteString("- **Redis** предпочтителен, когда:\n")
	sb.WriteString("  - Сообщения маленькие (до 10 КБ) и требуется минимальная задержка.\n")
	sb.WriteString("  - Высокая пропускная способность критичнее гарантий доставки.\n")
	sb.WriteString("- **RabbitMQ** следует выбрать, если:\n")
	sb.WriteString("  - Важна надёжность: сообщения не должны теряться.\n")
	sb.WriteString("  - Размер сообщений большой (более 10 КБ) или нагрузка неравномерная.\n")
	sb.WriteString("- Для сценариев с жёсткими требованиями к latency на малых payload лучше Redis.\n")
	sb.WriteString("- Для систем, где потеря данных недопустима — RabbitMQ.\n")

	return sb.String()
}

func getResult(results []*TestResult, size, rate int) *TestResult {
	for _, tr := range results {
		if tr.MessageSize == size && tr.TargetRate == rate {
			return tr
		}
	}
	return nil
}