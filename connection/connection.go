package connection

import (
	"encoding/json"
	"net"
	"os"
	"strings"
	"time"

	"github.com/byuoitav/central-event-system/hub/base"
	"github.com/byuoitav/central-event-system/messenger"
	"github.com/byuoitav/common/log"
	"github.com/byuoitav/common/v2/events"
	"github.com/gorilla/websocket"
)

var (
	address  = os.Getenv("DB_ADDRESS")
	username = os.Getenv("DB_USERNAME")
	password = os.Getenv("DB_PASSWORD")
)

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
	messenger, nerr := messenger.BuildMessenger(os.Getenv("HUB_ADDRESS"), base.Messenger, 1000)
	if nerr != nil {
		log.L.Debugf("There was an error building the messenger: %s", nerr.Error())
	}
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
			k := struct {
				Key     string              `json:"Key"`
				Content deviceUpdateContent `json:"Content"`
			}{}
			err = json.Unmarshal([]byte(str), &k)
			if err != nil {
				log.L.Debugf("There was an error unmarshaling the json: %s", err)
				continue
			}
			hostnameSearch, err := net.LookupAddr(k.Content.IPAddress)
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
			if k.Content.Connected == true {
				connectedEvent := events.Event{
					Timestamp:        time,
					GeneratingSystem: name,
					TargetDevice:     deviceInfo,
					Key:              "oline",
					Value:            "Online",
				}
				// forwardEvent(connectedEvent)
				messenger.SendEvent(connectedEvent)
			} else if k.Content.Connected == false {
				connectedEvent := events.Event{
					Timestamp:        time,
					GeneratingSystem: name,
					TargetDevice:     deviceInfo,
					Key:              "oline",
					Value:            "Offline",
				}
				// forwardEvent(connectedEvent)
				messenger.SendEvent(connectedEvent)
			}
			addressEvent := events.Event{
				Timestamp:        time,
				GeneratingSystem: name,
				TargetDevice:     deviceInfo,
				Key:              "ip-adress",
				Value:            k.Content.IPAddress,
			}
			// forwardEvent(addressEvent)
			messenger.SendEvent(addressEvent)
		}
	}
}

func forwardEvent(e events.Event) {
	// FWD_ENPOINTS should be a string of all of the forwarding urls separated by commas w/o spaces
	// fwdEndpointsString := os.Getenv("FWD_ENDPOINTS")
	// fwdEndpoints := strings.Split(fwdEndpointsString, ",")
	// fmt.Printf(fwdEndpoints[0])
	messenger, nerr := messenger.BuildMessenger(os.Getenv("HUB_ADDRESS"), base.Messenger, 1000)
	if nerr != nil {
		log.L.Debugf("There was an error building the messenger: %s", nerr.Error())
	}

	messenger.SendEvent(e)

	// for _, i := range fwdEndpoints {

	// derek, err := json.Marshal(e)
	// if err != nil {
	// 	log.L.Debugf("There was an error marshaling the json: %s", err)
	// }
	// fmt.Printf("This is the body: %s\n", derek)
	// req, err := http.NewRequest("POST", i, bytes.NewReader(derek))
	// if err != nil {
	// 	log.L.Debugf("There was an error generating the for endpoint %s: %s", i, err)
	// }

	// client := &http.Client{}
	// response, err := client.Do(req)
	// if err != nil {
	// 	log.L.Debugf("There was an error forwarding the event to %s: %s", i, err)
	// }
	// log.L.Debugf("Response from posting to %s: %v", i, response)
	// }
}
