package internal

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"
)

const adminStatusEndpoint = "http://deye-solar-inverter/status.html"
const expectedDailyOutput float64 = 20

var yieldTodayPattern = regexp.MustCompile(`var webdata_today_e = "([^"]+)";`)
var yieldTotalPattern = regexp.MustCompile(`var webdata_total_e = "([^"]+)";`)

type Solar struct {
	client        *Client
	channelId     string
	adminUser     string
	adminPassword string
}

func NewSolar(client *Client, adminUser, adminPassword string) *Solar {
	channelId := os.Getenv("SOLAR_REPORT_CHANNEL_ID")

	if channelId == "" {
		panic("SOLAR_REPORT_CHANNEL_ID must be set")
	}

	return &Solar{
		client:        client,
		adminUser:     adminUser,
		adminPassword: adminPassword,
		channelId:     channelId,
	}
}

func (this *Solar) SendDailyReport() {
	metrics, err := this.querySolarMetrics()

	if err != nil {
		fmt.Println("Error querying soalr metrics:", err)
		return
	}

	fmt.Printf("[SOLAR] Metrics: %v\n", metrics)

	deltaYieldToday := metrics.today - expectedDailyOutput

	var summary string

	if deltaYieldToday < 0 {
		summary = fmt.Sprintf("☹️ Below expected daily yield of %.2f kWh", expectedDailyOutput)
	} else {
		summary = fmt.Sprintf("😁 Above expected daily yield by %.2f kWh", deltaYieldToday)
	}

	var todayFormatted = time.Now().Format("Monday, 2 Jan 2006")

	message := fmt.Sprintf(
		"*🔆 Solar Production Report, %s*\n\n"+
			summary+
			"\n\n"+
			"Today: %.2f kWh\n"+
			"Total: %.2f kWh\n\n"+
			"`DO NOT CLICK: %d`",
		todayFormatted,
		metrics.today,
		metrics.total,
		time.Now().Unix(),
	)

	fmt.Printf("[SOLAR] Message to be sent:\n%s", message)

	this.client.SendTextMessage(
		this.channelId,
		message,
	)
}

func (this *Solar) querySolarMetrics() (*struct {
	today float64
	total float64
}, error) {
	req, err := http.NewRequest("GET", adminStatusEndpoint, nil)
	if err != nil {
		panic(err)
	}

	req.SetBasicAuth(this.adminUser, this.adminPassword)
	res, err := http.DefaultClient.Do(req)

	if err != nil {
		panic(err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bad status %d querying status page", res.StatusCode)
	}

	defer res.Body.Close()

	rawBody, err := io.ReadAll(res.Body)

	if err != nil {
		return nil, err
	}

	html := string(rawBody)
	matches := yieldTodayPattern.FindStringSubmatch(html)

	if len(matches) != 2 {
		return nil, fmt.Errorf("Regex failed to match today's yield")
	}

	yieldToday, err := strconv.ParseFloat(matches[1], 64)

	if err != nil {
		return nil, fmt.Errorf("Failed to convert %s to float64: %w", matches[1], err)
	}

	matches = yieldTotalPattern.FindStringSubmatch(html)

	if len(matches) != 2 {
		return nil, fmt.Errorf("Regex failed to match total yield")
	}

	yieldTotal, err := strconv.ParseFloat(matches[1], 64)

	if err != nil {
		return nil, fmt.Errorf("Failed to convert %s to float64: %w", matches[1], err)
	}

	return &struct {
		today float64
		total float64
	}{
		today: yieldToday,
		total: yieldTotal,
	}, nil
}
