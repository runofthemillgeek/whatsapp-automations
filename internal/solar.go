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
const criticalBelowDailyOutput float64 = 10

var yieldTodayPattern = regexp.MustCompile(`var webdata_today_e = "([^"]+)";`)
var yieldTotalPattern = regexp.MustCompile(`var webdata_total_e = "([^"]+)";`)

type SolarMetrics struct {
	lastUpdatedAt time.Time
	today         float64
	total         float64
}

type Solar struct {
	client        *Client
	channelId     string
	adminUser     string
	adminPassword string
	metrics       SolarMetrics
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

func (this *Solar) CronSendDailyReport() {
	// TODO: Add retries?
	fmt.Println("[SOLAR] Cron job invoked to send daily report")
	defer fmt.Println("[SOLAR] Cron job completed to send daily report")

	if this.metrics.lastUpdatedAt.IsZero() {
		fmt.Println("[SOLAR] Got zero value for lastUpdatedAt, means either inverter is not working properly or there was a long power outage or network connectivity problems")

		this.client.SendTextMessage(
			this.channelId,
			messageHeaderLine()+
				"🔴 Couldn't collect any data today. Check logs, inverter and connectivity.",
		)

		return
	}

	fmt.Printf("[SOLAR] Metrics: %v\n", this.metrics)

	message := fmt.Sprintf(
		messageHeaderLine()+
			this.getSummary()+
			"\n\n"+
			"*Today: %.2f kWh*\n"+
			"Total: %.2f kWh\n\n"+
			"_Last updated: %s_\n\n"+
			messageDoNotClickLine(),
		this.metrics.today,
		this.metrics.total,
		time.Now().Format("03:04 PM, 2006-01-02"),
	)

	fmt.Printf("[SOLAR] Message to be sent:\n%s", message)

	this.client.SendTextMessage(
		this.channelId,
		message,
	)
}

func (this *Solar) getSummary() string {
	deltaYieldToday := this.metrics.today - expectedDailyOutput

	if this.metrics.today < criticalBelowDailyOutput {
		return fmt.Sprintf("⚠️ Yield was only %.2f kWh today, well below expected %.2f kWh. Check if this was due to bad weather/powercut or if panels need maintenance.", this.metrics.today, expectedDailyOutput)
	} else if deltaYieldToday < 0 {
		return fmt.Sprintf("☹️ Below expected daily yield of %.2f kWh", expectedDailyOutput)
	} else {
		return fmt.Sprintf("😁 Above expected daily yield by %.2f kWh", deltaYieldToday)
	}
}

func (this *Solar) CronCollectMetrics() {
	fmt.Println("[SOLAR] Cron job invoked to collect fresh metrics")
	defer fmt.Println("[SOLAR] Cron job completed to collect fresh metrics")

	// Reset if we rolled over to next day
	if !AreSameDay(time.Now(), this.metrics.lastUpdatedAt) {
		this.metrics = SolarMetrics{}
	}

	metrics, err := this.querySolarMetrics()

	if err != nil {
		fmt.Println("[SOLAR] Error querying solar metrics:", err)
		return
	}

	this.metrics = *metrics

	fmt.Printf("[SOLAR] Updated metrics: %+v\n", this.metrics)
}

func (this *Solar) querySolarMetrics() (*SolarMetrics, error) {
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

	return &SolarMetrics{
		lastUpdatedAt: time.Now(),
		today:         yieldToday,
		total:         yieldTotal,
	}, nil
}

func messageDoNotClickLine() string {
	return fmt.Sprintf("`DO NOT CLICK: %d`", time.Now().Unix())
}

func messageHeaderLine() string {
	return fmt.Sprintf("*🔆 Solar Production Report — %s*\n\n", time.Now().Format("2 Jan 2006"))
}
