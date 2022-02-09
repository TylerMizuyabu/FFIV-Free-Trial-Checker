package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/ilyakaznacheev/cleanenv"
	mailjet "github.com/mailjet/mailjet-apiv3-go"
	"gocloud.dev/server"
)

var mu sync.Mutex

const trialStatusPage = "https://freetrial.finalfantasyxiv.com/"
const trialUnavailableText = "FREE TRIAL TEMPORARILY UNAVAILABLE"

var mailingList []string = []string{}

type mailJetConfig struct {
	publicApiKey  string `env:MAIL_JET_PUBLIC_KEY`
	privateApiKey string `env:MAIL_JET_PRIVATE_KEY`
}

func main() {
	srv := server.New(http.DefaultServeMux, nil)

	c := colly.NewCollector()

	var mCfg mailJetConfig
	if err := cleanenv.ReadEnv(&mCfg); err != nil {
		log.Panic(err)
	}

	mailJetClient := mailjet.NewMailjetClient(mCfg.publicApiKey, mCfg.privateApiKey)

	c.OnRequest(func(r *colly.Request) {
		log.Println("Visiting", r.URL)
	})

	c.OnHTML("body > div.top > div > div:nth-child(3) > h3", func(e *colly.HTMLElement) {
		if e.Text != trialUnavailableText {
			sendEmails(mailJetClient)
		}
	})

	http.HandleFunc("/subscribe", func(w http.ResponseWriter, r *http.Request) {
		email := r.URL.Query().Get("email")
		if len(email) > 0 {
			switch r.Method {
			case http.MethodPost:
				w.WriteHeader(http.StatusCreated)
				addEmail(email)
				log.Printf("%v", mailingList)
			case http.MethodDelete:
				w.WriteHeader(http.StatusNoContent)
				removeEmail(email)
				log.Printf("%v", mailingList)
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		}
	})

	ticker := time.NewTicker(1 * time.Hour)
	done := make(chan bool)

	go func() {
		c.Visit(trialStatusPage)
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				c.Visit(trialStatusPage)
			}
		}
	}()

	log.Println("Starting server")
	if err := srv.ListenAndServe(":8000"); err != nil {
		done <- true
		log.Fatalf("%v", err)
	}
}

func sendEmails(mailJetClient *mailjet.Client) {
	mu.Lock()
	defer mu.Unlock()
	recipientList := []mailjet.RecipientV31{}
	for _, e := range mailingList {
		recipientList = append(recipientList, mailjet.RecipientV31{
			Email: e,
		})
	}
	var recipients mailjet.RecipientsV31 = recipientList
	log.Printf("%v", recipientList)
	_, err := mailJetClient.SendMailV31(
		&mailjet.MessagesV31{
			Info: []mailjet.InfoMessagesV31{
				{
					From: &mailjet.RecipientV31{
						Email: "tylermizuyabu@gmail.com",
						Name:  "Tyler",
					},
					To:       &recipients,
					Subject:  "FFIV Online Free Trial",
					TextPart: "FFIV Online Free Trial Sales Are Resuming!",
				},
			},
		},
	)
	if err != nil {
		log.Printf("Error sending emails:", err)
		return
	}
	mailingList = []string{}
}

func removeEmail(email string) {
	mu.Lock()
	defer mu.Unlock()
	index := -1
	for i, e := range mailingList {
		if e == email {
			index = i
			break
		}
	}
	if index != -1 {
		mailingList = append(mailingList[0:index], mailingList[index+1:]...)
	}
}

func addEmail(email string) {
	mu.Lock()
	defer mu.Unlock()
	for _, e := range mailingList {
		if e == email {
			return
		}
	}
	mailingList = append(mailingList, email)
}
