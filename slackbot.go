package slackbot

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// SlackBot is the main type for creating robots.
type SlackBot struct {
	Name         string
	token        string
	connectionID string
	HearMap      map[string]Action
	RespondMap   map[string]Action
	webSocket    *websocket.Conn
	IsMuted      bool
}

// ActionHandler is a callback for an action
type ActionHandler func(*SlackBot, Message)

// Action is the type for slackbot actions
type Action struct {
	Handler         ActionHandler
	Pattern         string
	FriendlyPattern string
	Description     string
}

// New creates a new SlackBot with the given settings.
func New(name, token string) *SlackBot {
	return &SlackBot{
		Name:       name,
		token:      token,
		HearMap:    make(map[string]Action),
		RespondMap: make(map[string]Action),
		IsMuted:    false,
	}
}

// HearAction will call the action everytime the robot sees a message.
func (bot *SlackBot) HearAction(action Action) {
	bot.HearMap[action.Pattern] = action
}

// RespondAction will call the action everytime the robot sees a message.
func (bot *SlackBot) RespondAction(action Action) {
	bot.RespondMap[action.Pattern] = action
}

// Hear will call the action everytime the robot sees a message.
func (bot *SlackBot) Hear(pattern string, handler ActionHandler, friendlyPattern, description string) {
	bot.HearAction(Action{
		Handler:         handler,
		Pattern:         pattern,
		FriendlyPattern: friendlyPattern,
		Description:     description,
	})
}

// Respond will call the action everytime the robot sees a mention.
func (bot *SlackBot) Respond(pattern string, handler ActionHandler, friendlyPattern, description string) {
	bot.RespondAction(Action{
		Handler:         handler,
		Pattern:         pattern,
		FriendlyPattern: friendlyPattern,
		Description:     description,
	})
}

// Say will send the specified text.
func (bot *SlackBot) Say(m Message, message string) {
	if !bot.IsMuted {
		go func(m Message) {
			m.Text = message
			postMessage(bot.webSocket, m)
		}(m)
	}
}

// Mute will mute the bot for s seconds.
func (bot *SlackBot) Mute(s int) {
	go func(bot *SlackBot) {
		fmt.Printf("Muting for %d seconds\n", s)
		bot.IsMuted = true
		time.Sleep(time.Second * time.Duration(s))
		bot.IsMuted = false
		fmt.Println("Unmuting")
	}(bot)
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

		// see if we're mentioned
		if m.Type == "message" {
			for pattern, action := range bot.HearMap {
				if matched, _ := regexp.MatchString(pattern, m.Text); matched {
					action.Handler(bot, m)
				}
			}
			if strings.HasPrefix(m.Text, "<@"+bot.connectionID+">") {
				hadMatch := false
				for pattern, action := range bot.RespondMap {
					//  TODO: remove the mention from the message?
					if matched, _ := regexp.MatchString(pattern, m.Text); matched {
						hadMatch = true
						action.Handler(bot, m)
					}
				}
				if !hadMatch {
					bot.Say(m, "Huh?")
				}
			}
		}
	}
}
