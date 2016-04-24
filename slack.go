package slackbot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// response of the Slack API rtm.start.
type responseRtmStart struct {
	Ok    bool         `json:"ok"`
	Error string       `json:"error"`
	URL   string       `json:"url"`
	Self  responseSelf `json:"self"`
}

// Self representation from rtm.start.
type responseSelf struct {
	ID string `json:"id"`
}

// slackStart uses the slack api to get a web socket url and a user id.
func slackStart(token string) (wsurl, id string, err error) {
	url := fmt.Sprintf("https://slack.com/api/rtm.start?token=%s", token)
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("API request failed with code %d", resp.StatusCode)
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return
	}
	var respObj responseRtmStart
	err = json.Unmarshal(body, &respObj)
	if err != nil {
		return
	}

	if !respObj.Ok {
		err = fmt.Errorf("Slack error: %s", respObj.Error)
		return
	}

	wsurl = respObj.URL
	id = respObj.Self.ID
	return
}

// IncommingMessage is a generic message from slack
type IncommingMessage struct {
	ID        uint64          `json:"id"`
	Type      string          `json:"type"`
	Channel   json.RawMessage `json:"channel"`
	Text      string          `json:"text"`
	User      json.RawMessage `json:"user"`
	ChannelID string
	UserID    string
}

// Message is the default slack message struct.
type Message struct {
	ID          uint64       `json:"id"`
	Type        string       `json:"type"`
	Channel     string       `json:"channel"`
	Text        string       `json:"text"`
	User        string       `json:"user"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Attachment for outgoing messages
type Attachment struct {
	Fallback   *string `json:"fallback,omitempty"`
	Color      *string `json:"color,omitempty"`
	Text       *string `json:"text,omitempty"`
	PreText    *string `json:"pretext,omitempty"`
	AuthorName *string `json:"author_name,omitempty"`
	AuthorLink *string `json:"author_link,omitempty"`
	AuthorIcon *string `json:"author_icon,omitempty"`
	Title      *string `json:"title,omitempty"`
	TitleLink  *string `json:"title_link,omitempty"`
	// "fields": [
	//     {
	//         "title": "Priority",
	//         "value": "High",
	//         "short": false
	//     }
	// ],
	// "image_url": "http://my-website.com/path/to/image.jpg",
	// "thumb_url": "http://example.com/path/to/thumb.png"
}

// ChannelObject represents the channel when it's not a string.
type ChannelObject struct {
	ID string `json:"id"`
}

// UserObject represents a user for messages where it's not a string.
type UserObject struct {
	ID string `json:"id"`
}

func getMessage(ws *websocket.Conn) (m Message, err error) {
	var im IncommingMessage
	err = ws.ReadJSON(&im)
	fmt.Println("MESSAGE:", im)
	if err == nil && len(im.Channel) > 0 {
		// Try a string Channel
		var cid string
		err = json.Unmarshal(im.Channel, &cid)
		if err == nil {
			im.ChannelID = cid
		} else {
			var cobj ChannelObject
			err = json.Unmarshal(im.Channel, &cobj)
			if err == nil {
				im.ChannelID = cobj.ID
			}
		}
	}
	if err == nil && len(im.User) > 0 {
		// Try a string User
		var uid string
		err = json.Unmarshal(im.User, &uid)
		if err == nil {
			im.UserID = uid
		} else {
			var uobj UserObject
			err = json.Unmarshal(im.User, &uobj)
			if err == nil {
				im.UserID = uobj.ID
			}
		}
	}
	m = Message{
		ID:      im.ID,
		Type:    im.Type,
		Channel: im.ChannelID,
		Text:    im.Text,
		User:    im.UserID,
	}
	return
}

var counter uint64

// Send a message through the web socket.
func postMessage(ws *websocket.Conn, m Message) error {
	m.ID = atomic.AddUint64(&counter, 1)
	return ws.WriteJSON(m)
}

// Starts a websocket-based Real Time API session and return the websocket.
func slackConnect(token string) (*websocket.Conn, string) {
	wsurl, id, err := slackStart(token)
	if err != nil {
		log.Fatal(err)
	}

	ws, _, err := websocket.DefaultDialer.Dial(wsurl, nil)
	if err != nil {
		log.Fatal(err)
	}

	return ws, id
}
