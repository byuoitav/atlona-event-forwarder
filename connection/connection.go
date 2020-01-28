package connection

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
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
	address            = os.Getenv("DB_ADDRESS")
	username           = os.Getenv("DB_USERNAME")
	password           = os.Getenv("DB_PASSWORD")
	eventProcessorHost = os.Getenv("EVENT_PROCESSOR_HOST")
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

	//MB 1-28-2020 - changing to just send the event over HTTP
	/*
		messenger, nerr := messenger.BuildMessenger(os.Getenv("HUB_ADDRESS"), base.Messenger, 1000)
		if nerr != nil {
			log.L.Debugf("There was an error building the messenger: %s", nerr.Error())
		}
	*/
	for {
		_, bytes, err := ws.ReadMessage()
		if err != nil {
			log.L.Debugf("Unable to read message from connection %s: %s", ws, err)
		}

		str := string(bytes)
		str = strings.TrimSpace(str)

		time := time.Now()

		log.L.Debugf("Message Received: %s", str)

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
			hostname := ""
			info := []string{"", ""}
			if err != nil {
				log.L.Debugf("There was an error retrieving the hostname: %s", err)
			} else {
				hostname = strings.Trim(hostnameSearch[0], ".byu.edu.")
				info = strings.Split(hostname, "-")
			}

			roomInfo := events.BasicRoomInfo{
				BuildingID: info[0],
				RoomID:     info[1],
			}

			deviceInfo := events.BasicDeviceInfo{
				BasicRoomInfo: roomInfo,
				DeviceID:      name,
			}

			if k.Content.Connected == true {
				connectedEvent := events.Event{
					Timestamp:        time,
					GeneratingSystem: name,
					EventTags:        []string{"heartbeat", "auto-generated"},
					TargetDevice:     deviceInfo,
					Key:              "online",
					Value:            "Online",
					Data:             hostname,
				}
				log.L.Debugf("Sending event: %+v", connectedEvent)
				sendEvent(connectedEvent)
			} else if k.Content.Connected == false {
				connectedEvent := events.Event{
					Timestamp:        time,
					GeneratingSystem: name,
					TargetDevice:     deviceInfo,
					Key:              "online",
					Value:            "Offline",
					Data:             hostname,
				}
				log.L.Debugf("Sending event: %+v", connectedEvent)
				sendEvent(connectedEvent)
			}

			addressEvent := events.Event{
				Timestamp:        time,
				GeneratingSystem: name,
				TargetDevice:     deviceInfo,
				Key:              "ip-address",
				Value:            k.Content.IPAddress,
				Data:             hostname,
			}

			log.L.Debugf("Sending event: %+v", addressEvent)
			sendEvent(addressEvent)
		}
	}
}

func sendEvent(x events.Event) error {
	// marshal request if not already an array of bytes
	reqBody, err := json.Marshal(x)
	if err != nil {
		log.L.Debugf("Unable to marshal event %v, error:%v", x, err)
		return err
	}

	eventProcessorHostList := strings.Split(eventProcessorHost, ",")

	for _, hostName := range eventProcessorHostList {

		// create the request
		log.L.Debugf("Sending to address %s", hostName)

		req, err := http.NewRequest("POST", hostName, bytes.NewReader(reqBody))
		if err != nil {
			log.L.Debugf("Unable to post body %v, error:%v", reqBody, err)
			//return []byte{}, nerr.Translate(err)
		}

		//no auth needed for state parser
		//req.SetBasicAuth(username, password)

		// add headers
		req.Header.Add("content-type", "application/json")

		client := http.Client{
			Timeout: 5 * time.Second,
		}

		resp, err := client.Do(req)
		if err != nil {
			log.L.Debugf("error sending request: %v", err)
		} else {
			defer resp.Body.Close()
			// read the resp
			respBody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.L.Debugf("error reading body: %v", err)
			} else {

				// check resp code
				if resp.StatusCode/100 != 2 {
					log.L.Debugf("non 200 reponse code received. code: %v, body: %s", resp.StatusCode, respBody)
				}
			}
		}
	}

	return nil
}
