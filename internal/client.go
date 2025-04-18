package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
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
const allowlistFileName = "allowlist.json"

type Client struct {
	WAClient            *whatsmeow.Client
	message             string
	autoResponseTimeMap map[string]string
	allowlist           []string
}

func NewClient(waClient *whatsmeow.Client) *Client {
	autoResponseTimeMap := map[string]string{}
	allowlist := []string{}

	fileInfo, _ := os.Stat(autoResponseTimeMapJsonFileName)

	if fileInfo != nil {
		bytes, err := os.ReadFile(autoResponseTimeMapJsonFileName)

		if err != nil {
			panic(fmt.Errorf("error reading %s: %w", autoResponseTimeMapJsonFileName, err))
		}

		err = json.Unmarshal(bytes, &autoResponseTimeMap)

		if err != nil {
			panic(err)
		}
	}

	fileInfo, _ = os.Stat(allowlistFileName)

	if fileInfo != nil {
		bytes, err := os.ReadFile(allowlistFileName)

		if err != nil {
			panic(fmt.Errorf("error reading %s: %w", allowlistFileName, err))
		}

		err = json.Unmarshal(bytes, &allowlist)

		if err != nil {
			panic(err)
		}
	}

	return &Client{
		WAClient:            waClient,
		message:             waautoresponder.AutoResponderMessage,
		autoResponseTimeMap: autoResponseTimeMap,
		allowlist:           allowlist,
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

	bytes, err := json.MarshalIndent(client.autoResponseTimeMap, "", "  ")

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
		chatUserId := v.Info.Chat.User

		if v.Info.IsFromMe {
			return
		}

		if !slices.Contains(client.allowlist, chatUserId) {
			if v.Info.IsGroup {
				return
			}

			// Ignore business chats
			if v.Info.VerifiedName != nil {
				return
			}
		}

		if client.hasAutoRespondedWithinSameDay(chatUserId) {
			fmt.Printf("Already responded to user %s, skipping\n", chatUserId)
			return
		}

		if time.Now().Sub(v.Info.Timestamp).Minutes() >= 5 {
			fmt.Printf("Message from %s older than 5 minutes, skipping\n", chatUserId)
			return
		}

		time.Sleep(2 * time.Duration(rand.IntN(3)) * time.Second)

		client.WAClient.SendChatPresence(v.Info.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaText)

		time.Sleep(2 * time.Duration(rand.IntN(3)) * time.Second)

		msg := proto.String(client.message + "\n\nIgnore this random number: `" + strconv.FormatInt(time.Now().Unix(), 10) + "`")

		imageResp, err := client.WAClient.Upload(context.Background(), waautoresponder.Bernie, whatsmeow.MediaImage)

		if err == nil {
			client.WAClient.SendMessage(
				context.Background(),
				v.Info.Chat,
				&waE2E.Message{
					ImageMessage: &waE2E.ImageMessage{
						Caption:  msg,
						Mimetype: proto.String("image/jpeg"),

						URL:           &imageResp.URL,
						DirectPath:    &imageResp.DirectPath,
						MediaKey:      imageResp.MediaKey,
						FileEncSHA256: imageResp.FileEncSHA256,
						FileSHA256:    imageResp.FileSHA256,
						FileLength:    &imageResp.FileLength,
					},
				})
		} else {
			client.WAClient.SendMessage(
				context.Background(),
				v.Info.Chat,
				&waE2E.Message{
					Conversation: msg,
				},
			)
		}

		client.WAClient.SendChatPresence(v.Info.Chat, types.ChatPresencePaused, types.ChatPresenceMediaText)

		fmt.Printf("Sent autoresponder message to user %s\n", chatUserId)

		client.updateAutoResponseTime(chatUserId)
	}
}

func (client *Client) SendTextMessage(chatId string, message string) {
	chatJID, err := types.ParseJID(chatId)

	if err != nil {
		fmt.Println("Error parsing JID:", err)
		return
	}

	time.Sleep(2 * time.Duration(rand.IntN(3)) * time.Second)
	client.WAClient.SendChatPresence(chatJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	time.Sleep(2 * time.Duration(rand.IntN(3)) * time.Second)

	client.WAClient.SendMessage(
		context.Background(),
		chatJID,
		&waE2E.Message{
			Conversation: proto.String(message),
		},
	)

	client.WAClient.SendChatPresence(chatJID, types.ChatPresencePaused, types.ChatPresenceMediaText)
}
