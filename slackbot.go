package slackbot

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
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
type ActionHandler func(*SlackBot, *ActionContext)

// Action is the type for slackbot actions
type Action struct {
	Handler         ActionHandler
	Pattern         string
	FriendlyPattern string
	Description     string
	regexp          *regexp.Regexp
}

// ActionContext with information will be passed to all actions
type ActionContext struct {
	Action  Action
	Matches [][]string
	Message Message
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

// Compile regular expression in actions
func (a *Action) Compile() {
	re, err := regexp.Compile(a.Pattern)
	if err != nil {
		log.Println(err)
		panic(fmt.Sprintf("ERROR compiling regexp: %s\n\t%v", a.Pattern, err))
	}
	a.regexp = re
}

// HearAction will call the action everytime the robot sees a message.
func (bot *SlackBot) HearAction(action Action) {
	action.Compile()
	bot.HearMap[action.Pattern] = action
}

// RespondAction will call the action everytime the robot sees a message.
func (bot *SlackBot) RespondAction(action Action) {
	action.Compile()
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

// PostMessagePayload for outgoing messages
type PostMessagePayload struct {
	Token       string       `json:"token"`
	Type        string       `json:"type"`
	Channel     string       `json:"channel"`
	Text        string       `json:"text"`
	User        string       `json:"user"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// PostMessage will send the specified text.
func (bot *SlackBot) PostMessage(m Message) error {
	// Say will send the specified text.
	// Use web api to post message so we can support attachments.
	apiURL := "https://slack.com/api/chat.postMessage"
	strAttachments, _ := json.Marshal(m.Attachments)

	resp, err := http.PostForm(apiURL, url.Values{
		"token":       {bot.token},
		"parse":       {"full"},
		"channel":     {m.Channel},
		"text":        {m.Text},
		"attachments": {string(strAttachments)},
	})
	defer resp.Body.Close()
	// body, err := ioutil.ReadAll(resp.Body)
	return err
}

// Say will send the specified text.
func (bot *SlackBot) Say(m Message, message string) {
	if !bot.IsMuted {
		// TODO: gorilla websockets don't support concurrent writes...
		// go func(m Message) {
		m.Text = message
		// postMessage(bot.webSocket, m)
		bot.PostMessage(m)
		// }(m)
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
		if m.Type == "message" && m.User != bot.connectionID {
			for pattern, action := range bot.HearMap {
				if matched, _ := regexp.MatchString(pattern, m.Text); matched {
					bot.handleAction(action, m)
				}
			}
			if strings.HasPrefix(m.Text, "<@"+bot.connectionID+">") {
				hadMatch := false
				for pattern, action := range bot.RespondMap {
					//  TODO: remove the mention from the message?
					if matched, _ := regexp.MatchString(pattern, m.Text); matched {
						hadMatch = true
						bot.handleAction(action, m)
					}
				}
				if !hadMatch {
					bot.Say(m, "uhhhmmm...")
				}
			}
		}
	}
}

func (bot *SlackBot) handleAction(action Action, m Message) {
	matches := action.regexp.FindAllStringSubmatch(m.Text, -1)
	context := ActionContext{
		Action:  action,
		Matches: matches,
		Message: m,
	}
	action.Handler(bot, &context)
}
