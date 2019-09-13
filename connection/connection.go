package connection

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/byuoitav/common/log"
	"github.com/byuoitav/common/v2/events"
	"github.com/gorilla/websocket"
)

var (
	address  = os.Getenv("DB_ADDRESS")
	username = os.Getenv("DB_USERNAME")
	password = os.Getenv("DB_PASSWORD")
)

type key struct {
	Key     string      `json:"Key"`
	Content interface{} `json:"Content"`
}

type deviceUpdateContent struct {
	ID         string `json:"Id"`
	Connected  bool   `json:"Connected"`
	IPAddress  string `json:"IPAddress"`
	Connecting bool   `json:"Connecting"`
	CanPing    bool   `json:"CanPing"`
	Aliases    string `json:"Aliases"`
}

//ReadMessage will read the messages from each websocket
func ReadMessage(ws *websocket.Conn, name string) {
	for {
		_, bytes, err := ws.ReadMessage()
		if err != nil {
			log.L.Debugf("Unable to read message from connection %s: %s", ws, err)
		}

		str := string(bytes)
		str = strings.TrimSpace(str)

		time := time.Now()

		switch {
		//these are other events that get sent but we don't use them
		case strings.Contains(str, "\"Key\":\"KeepAliveSocket\""):
		case strings.Contains(str, "\"Key\":\"Store\""):
		case strings.Contains(str, "\"Key\":\"DeviceUpdate\""):
			var k key
			k.Content = deviceUpdateContent{}
			err = json.Unmarshal([]byte(str), &k)
			if err != nil {
				log.L.Debugf("There was an error unmarshaling the json: %s", err)
				continue
			}
			content := k.Content.(deviceUpdateContent)
			hostnameSearch, err := net.LookupAddr(content.IPAddress)
			if err != nil {
				log.L.Debugf("There was an error retrieving the hostname: %s", err)
			}
			hostname := strings.Trim(hostnameSearch[0], ".byu.edu.")
			info := strings.Split(hostname, "-")
			roomInfo := events.BasicRoomInfo{
				BuildingID: info[0],
				RoomID:     info[1],
			}
			deviceInfo := events.BasicDeviceInfo{
				BasicRoomInfo: roomInfo,
				DeviceID:      hostname,
			}
			log.L.Debugf("Here is the response: %s", str)
			if content.Connected == true {
				connectedEvent := events.Event{
					Timestamp:        time,
					GeneratingSystem: name,
					TargetDevice:     deviceInfo,
					Key:              "online",
					Value:            "Online",
				}
				forwardEvent(connectedEvent)
			} else if content.Connected == false {
				connectedEvent := events.Event{
					Timestamp:        time,
					GeneratingSystem: name,
					TargetDevice:     deviceInfo,
					Key:              "online",
					Value:            "Offline",
				}
				forwardEvent(connectedEvent)
			}
			addressEvent := events.Event{
				Timestamp:        time,
				GeneratingSystem: name,
				TargetDevice:     deviceInfo,
				Key:              "ip-address",
				Value:            content.IPAddress,
			}
			forwardEvent(addressEvent)
		}
	}
}

func forwardEvent(e events.Event) {
	// FWD_ENPOINTS should be a string of all of the forwarding urls separated by commas w/o spaces
	fwdEndpointsString := os.Getenv("FWD_ENDPOINTS")
	fwdEndpoints := strings.Split(fwdEndpointsString, ",")

	// now we need to determine what messages we care about and how to send them on
	// post to smee using the /event endpoint
	for _, i := range fwdEndpoints {
		derek, err := json.Marshal(e)
		if err != nil {
			log.L.Debugf("There was an error marshaling the json: %s", err)
		}
		response, err := http.NewRequest("POST", i, bytes.NewReader(derek))
		if err != nil {
			log.L.Debugf("There was an error forwarding the event to %s: %s", i, err)
		}
		log.L.Debugf("Response from posting to %s: %s", i, response)
	}
}
