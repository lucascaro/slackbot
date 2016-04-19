package slackbot

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"golang.org/x/net/websocket"
)

// SlackBot is the main type for creating robots.
type SlackBot struct {
	Name         string
	token        string
	connectionID string
	hearMap      map[string]Action
	respondMap   map[string]Action
	webSocket    *websocket.Conn
}

// Action is the type for slackbot actions
type Action func(*SlackBot, Message)

// New creates a new SlackBot with the given settings.
func New(name, token string) *SlackBot {
	return &SlackBot{
		Name:       name,
		token:      token,
		hearMap:    make(map[string]Action),
		respondMap: make(map[string]Action),
	}
}

// Hear will call the action everytime the robot sees a message.
func (bot *SlackBot) Hear(pattern string, action Action) {
	bot.hearMap[pattern] = action
	fmt.Println(bot.hearMap)
}

// Respond will call the action everytime the robot sees a mention.
func (bot *SlackBot) Respond(pattern string, action Action) {
	bot.respondMap[pattern] = action
}

// Say will send the specified text.
func (bot *SlackBot) Say(m Message, message string) {
	go func(m Message) {
		m.Text = message
		postMessage(bot.webSocket, m)
	}(m)
}

// Connect will connect the robot and start the main loop.
func (bot *SlackBot) Connect() {
	// start a websocket-based Real Time API session
	bot.webSocket, bot.connectionID = slackConnect(bot.token)
	fmt.Printf("%s ready, ^C exits\n", bot.Name)

	for {
		// read each incoming message
		m, err := getMessage(bot.webSocket)
		if err != nil {
			log.Fatal(err)
		}

		log.Println(m)
		// see if we're mentioned
		if m.Type == "message" {
			for pattern, action := range bot.hearMap {
				if matched, _ := regexp.MatchString(pattern, m.Text); matched {
					action(bot, m)
				}
			}
			if strings.HasPrefix(m.Text, "<@"+bot.connectionID+">") {
				hadMatch := false
				for pattern, action := range bot.respondMap {
					//  TODO: remove the mention from the message?
					if matched, _ := regexp.MatchString(pattern, m.Text); matched {
						hadMatch = true
						action(bot, m)
					}
				}
				if !hadMatch {
					bot.Say(m, "Huh?")
				}
			}
		}
	}
}
