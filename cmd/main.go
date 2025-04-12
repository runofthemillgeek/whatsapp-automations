package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/robfig/cron"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"

	waautoresponder "git.sangeeth.dev/wa-autoresponder"
	"git.sangeeth.dev/wa-autoresponder/internal"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	fmt.Println("Auto responder message body:")
	fmt.Println(waautoresponder.AutoResponderMessage)

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

	deyeUser := os.Getenv("DEYE_USER")
	deyePassword := os.Getenv("DEYE_PASSWORD")

	if deyeUser == "" || deyePassword == "" {
		panic("Deye user/password must be provided")
	}

	solar := internal.NewSolar(client, deyeUser, deyePassword)

	myCron := cron.New()
	myCron.AddFunc("0 0 19 * * *", solar.SendDailyReport)
	myCron.Start()

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	myCron.Stop()
	client.Disconnect()
}
