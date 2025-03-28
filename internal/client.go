package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"time"

	waautoresponder "git.sangeeth.dev/wa-autoresponder"
	"github.com/mdp/qrterminal"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

const autoResponseTimeMapJsonFileName = "auto-response-time-map.json"

type Client struct {
	WAClient            *whatsmeow.Client
	message             string
	autoResponseTimeMap map[string]string
}

func NewClient(waClient *whatsmeow.Client) *Client {
	autoResponseTimeMap := map[string]string{}

	fileInfo, _ := os.Stat(autoResponseTimeMapJsonFileName)

	if fileInfo != nil {
		if fileInfo.IsDir() {
			panic(fmt.Errorf("expected %s to be a file, but found dir", autoResponseTimeMapJsonFileName))
		}

		bytes, err := os.ReadFile(autoResponseTimeMapJsonFileName)

		if err != nil {
			panic(fmt.Errorf("error reading %s: %w", autoResponseTimeMapJsonFileName, err))
		}

		err = json.Unmarshal(bytes, &autoResponseTimeMap)

		if err != nil {
			panic(err)
		}
	}

	return &Client{
		WAClient:            waClient,
		message:             waautoresponder.AutoResponderMessage,
		autoResponseTimeMap: autoResponseTimeMap,
	}
}

func (client *Client) Register() {
	client.WAClient.AddEventHandler(client.eventHandler)
}

func (myClient *Client) Connect() {
	client := myClient.WAClient

	if client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(context.Background())
		err := client.Connect()
		if err != nil {
			panic(err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				// Render the QR code here
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				// or just manually `echo 2@... | qrencode -t ansiutf8` in a terminal
				fmt.Println("QR code:", evt.Code)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		// Already logged in, just connect
		err := client.Connect()
		if err != nil {
			panic(err)
		}
	}
}

func (client *Client) Disconnect() {
	client.WAClient.Disconnect()
}

func (client *Client) hasAutoRespondedWithinSameDay(userId string) bool {
	if rawLastResponseTime, exists := client.autoResponseTimeMap[userId]; exists {
		parsedLastResponseTime, error := time.Parse(time.RFC3339, rawLastResponseTime)
		if error != nil {
			fmt.Fprintf(os.Stderr, "Map has time stored in invalid format, expected RFC3339. Raw value is %+v\n", rawLastResponseTime)
			return false
		}

		// If we already responded today, not need to send the same spiel again
		if AreSameDay(parsedLastResponseTime, time.Now()) {
			fmt.Printf("Already responded to user %s, skipping\n", userId)
			return true
		}
	}

	return false
}

func (client *Client) updateAutoResponseTime(userId string) {
	client.autoResponseTimeMap[userId] = time.Now().Format(time.RFC3339)

	bytes, err := json.Marshal(client.autoResponseTimeMap)

	if err != nil {
		panic(err)
	}

	if err = os.WriteFile(autoResponseTimeMapJsonFileName, bytes, 0660); err != nil {
		panic(err)
	}
}

func (client *Client) eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		if v.Info.IsGroup {
			return
		}

		if v.Info.IsFromMe {
			return
		}

		// Ignore business chats
		if v.Info.VerifiedName != nil {
			return
		}

		chatUserId := v.Info.Chat.User

		if client.hasAutoRespondedWithinSameDay(chatUserId) {
			fmt.Printf("Already responded to user %s, skipping\n", chatUserId)
			return
		}

		time.Sleep(2 * time.Duration(rand.IntN(3)) * time.Second)

		client.WAClient.SendChatPresence(v.Info.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaText)

		time.Sleep(2 * time.Duration(rand.IntN(3)) * time.Second)

		client.WAClient.SendMessage(
			context.Background(),
			v.Info.Chat,
			&waE2E.Message{
				Conversation: proto.String(client.message + "\n\nIgnore this random number: `" + strconv.FormatInt(time.Now().UnixMilli(), 10) + "`"),
			},
		)

		client.WAClient.SendChatPresence(v.Info.Chat, types.ChatPresencePaused, types.ChatPresenceMediaText)

		fmt.Printf("Sent autoresponder message to user %s\n", chatUserId)

		client.updateAutoResponseTime(chatUserId)
	}
}
