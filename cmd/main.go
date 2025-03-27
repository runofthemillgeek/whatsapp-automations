package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"

	"git.sangeeth.dev/wa-autoresponder/internal"

	_ "github.com/mattn/go-sqlite3"
)

var lastResponseTimeMap map[string]string = map[string]string{}

func areSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()

	return y1 == y2 && m1 == m2 && d1 == d2
}

func main() {
	// |------------------------------------------------------------------------------------------------------|
	// | NOTE: You must also import the appropriate DB connector, e.g. github.com/mattn/go-sqlite3 for SQLite |
	// |------------------------------------------------------------------------------------------------------|
	autoResponderMessageBytes, err := os.ReadFile("message.md")

	if err != nil {
		panic(fmt.Errorf("error reading message.md: %w", err))
	}

	autoResponderMessage := string(autoResponderMessageBytes)

	fmt.Println("Auto responder message body:")
	fmt.Println(autoResponderMessage)

	dbLog := waLog.Stdout("Database", "DEBUG", true)
	container, err := sqlstore.New("sqlite3", "file:whatsapp.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}

	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}

	clientLog := waLog.Stdout("Client", "DEBUG", true)
	waClient := whatsmeow.NewClient(deviceStore, clientLog)
	waClient.SendPresence(types.PresenceAvailable)

	client := internal.NewClient(waClient)
	client.Register()
	client.Connect()

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
}
