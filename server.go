package main

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"github.com/byuoitav/atlona-event-forwarder/connection"
	"github.com/gorilla/websocket"

	"github.com/byuoitav/common/db/couch"
	"github.com/byuoitav/common/log"
	"github.com/byuoitav/common/structs"
)

var (
	address  = os.Getenv("DB_ADDRESS")
	username = os.Getenv("DB_USERNAME")
	password = os.Getenv("DB_PASSWORD")
	Conns    map[string]*websocket.Conn
)

func init() {
	if len(address) == 0 || len(username) == 0 || len(password) == 0 {
		log.L.Fatalf("One of DB_ADDRESS, DB_USERNAME, DB_PASSWORD is not set. Failing...")
	}
}

func main() {
	log.SetLevel("debug")

	db := couch.NewDB(address, username, password)
	agwList, err := db.GetDevicesByType("AtlonaGateway")
	// log.L.Debugf("This is the name of agwlist: %s", agwList[0].ID)

	if err != nil {
		log.L.Fatalf("There was an error getting the AGWList: %v", err)
	}
	// fmt.Printf("Length of list and contents: %d %s\n", len(agwList), agwList[0].Address)

	Conns = make(map[string]*websocket.Conn)
	for _, i := range agwList {
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
		Conns[i.ID] = ws
	}

	//subscribe
	for i := range Conns {
		err = Conns[i].WriteMessage(websocket.BinaryMessage, []byte(`{"callBackId":4,"data":{"action":"SetCurrentPage","state":"{\"Page\":\"roomModifyDevices\"}","controller":"App"}}`))
		if err != nil {
			log.L.Debugf("unable to read message: %s", err)
		}

		go connection.ReadMessage(Conns[i], i)
	}

	for {
		//wait some time
		log.L.Debugf("Waiting to check for list changes")
		time.Sleep(5 * time.Minute)

		log.L.Debugf("Checking AGWList for changes")

		db := couch.NewDB(address, username, password)

		newAGWList, err := db.GetDevicesByType("AtlonaGateway")
		if err != nil {
			log.L.Debugf("there was an issue getting the AGWList: %v", err)
		}

		//check to see if the length is different
		if len(Conns) < len(newAGWList) {
			newList := make([]structs.Device, len(agwList))
			copy(newList, agwList)
			log.L.Debugf("comparing the list with the map to find the new one")

			for i := 0; i < len(newList); i++ {
				//for each object in newList check to see if it exists in the map of conns already
				new := newList[i]
				match := false

				for j := range Conns {

					if new.ID == j {
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
					Conns[new.ID] = ws
					err = ws.WriteMessage(websocket.BinaryMessage, []byte(`{"callBackId":4,"data":{"action":"SetCurrentPage","state":"{\"Page\":\"roomModifyDevices\"}","controller":"App"}}`))
					if err != nil {
						log.L.Debugf("unable to read message: %s", err)
					}

					go connection.ReadMessage(ws, new.ID)
				}
			}
		} else if len(Conns) > len(newAGWList) {
			var found bool
			for i := range Conns {
				found = false
				for _, x := range newAGWList {
					if i == x.ID {
						found = true
						break
					}
				}
				if found == false {
					delete(Conns, i)
				}
			}
		}
	}
}