package connection

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/byuoitav/common/db/couch"
	"github.com/byuoitav/common/log"
	"github.com/byuoitav/common/structs"
	"github.com/gorilla/websocket"
)

var (
	address  = os.Getenv("DB_ADDRESS")
	username = os.Getenv("DB_USERNAME")
	password = os.Getenv("DB_PASSWORD")
)

//Conns is a map of ID to websocket
var Conns map[string]*websocket.Conn

//MonitorAGWList monitors the list and updates it when a new gateway has been added
func MonitorAGWList(currentAGWList []structs.Device) {
	Conns = make(map[string]*websocket.Conn)
	for _, i := range currentAGWList {
		dialer := &websocket.Dialer{}
		address := "ws://"
		address += i.Address
		address += "/ws"
		ws, resp, err := dialer.DialContext(context.TODO(), address, nil)
		if err != nil {
			log.L.Fatalf("unable to open websocket: %s", err)
		}
		defer resp.Body.Close()

		bytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.L.Fatalf("unable to read bytes: %s", bytes)
		}

		log.L.Debugf("response from opening websocket: %s", bytes)
		Conns[i.Name] = ws
	}
	for {
		//wait some time
		log.L.Debugf("Waiting to check for list changes")
		time.Sleep(5 * time.Minute)

		log.L.Debugf("Checking AGWList for changes")

		db := couch.NewDB(address, username, password)

		agwList, err := db.GetDevicesByType("AtlonaGateway")
		if err != nil {
			log.L.Debugf("there was an issue getting the AGWList: %v", err)
		}

		//check to see if the length is different
		if len(agwList) != len(currentAGWList) {
			newList := make([]structs.Device, len(agwList))
			copy(newList, agwList)
			log.L.Debugf("comparing the list with the map to find the new one")

			for i := 0; i < len(newList); i++ {
				//for each object in newList check to see if it exists in the map of conns already
				new := newList[i]
				match := false

				for j := range Conns {

					if new.Name == j {
						match = true
						break
					}
				}

				//if it can't be found, create a websocket and add it to the map
				if !match {
					dialer := &websocket.Dialer{}
					address := "ws://"
					address += new.Address
					address += "/ws"
					ws, resp, err := dialer.DialContext(context.TODO(), address, nil)
					if err != nil {
						log.L.Fatalf("unable to open websocket: %s", err)
					}
					defer resp.Body.Close()

					bytes, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						log.L.Fatalf("unable to read bytes: %s", bytes)
					}

					log.L.Debugf("response from opening websocket: %s", bytes)
					Conns[new.Name] = ws
					err = ws.WriteMessage(websocket.BinaryMessage, []byte(`{"callBackId":4,"data":{"action":"SetCurrentPage","state":"{\"Page\":\"roomModifyDevices\"}","controller":"App"}}`))
					if err != nil {
						log.L.Debugf("unable to read message: %s", err)
					}

					go ReadMessage(ws)
				}
			}
		}
	}
}

//ReadMessage will read the messages from each websocket and probably forward it whereever it needs to go
func ReadMessage(ws *websocket.Conn) {
	// FWD_ENPOINTS should be a string of all of the forwarding urls separated by commas w/o spaces
	fwdEndpointsString := os.Getenv("FWD_ENDPOINTS")
	fwdEndpoints := strings.Split(fwdEndpointsString, ",")
	for {
		_, bytes, err := ws.ReadMessage()
		if err != nil {
			log.L.Debugf("Unable to read message from connection %s: %s", ws, err)
		}

		str := string(bytes)
		str = strings.TrimSpace(str)
		log.L.Debugf("Here is the response: %s", str)

		//now we need to determine what messages we care about and how to send them on
		//post to smee using the /event endpoint
		for _, i := range fwdEndpoints {
			response, err := http.NewRequest("POST", i, strings.NewReader(str))
			if err != nil {
				log.L.Debugf("There was an error forwarding the event to Smee: %s", err)
			}
			log.L.Debugf("Response from posting to Smee: %s", response)
		}
	}
}
