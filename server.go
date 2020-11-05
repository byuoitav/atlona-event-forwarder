package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo"

	"github.com/byuoitav/atlona-event-forwarder/connection"
	"github.com/byuoitav/common/log"
	_ "github.com/go-kivik/couchdb/v3"
	kivik "github.com/go-kivik/kivik/v3"
)

var (
	address            = os.Getenv("DB_ADDRESS")
	username           = os.Getenv("DB_USERNAME")
	password           = os.Getenv("DB_PASSWORD")
	loglevel           = os.Getenv("LOG_LEVEL")
	eventProcessorHost = os.Getenv("EVENT_PROCESSOR_HOST")
	conns              map[string]*websocket.Conn
)

func init() {
	if len(address) == 0 || len(username) == 0 || len(password) == 0 || len(eventProcessorHost) == 0 {
		log.L.Fatalf("One of DB_ADDRESS, DB_USERNAME, DB_PASSWORD, EVENT_PROCESSOR_HOST is not set. Failing...")
	}
}

type device struct {
	ID      string `json:"_id"`
	Address string `json:"address"`
}

func main() {
	e := echo.New()

	e.GET("/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "healthy")
	})

	go e.Start(":9998")

	log.SetLevel(loglevel)

	url := strings.Trim(address, "https://")
	url = strings.Trim(url, "http://")

	url = fmt.Sprintf("https://%s:%s@%s", username, password, url)
	client, err := kivik.New("couch", url)
	if err != nil {
		log.L.Fatalf("unable to build client: %w", err)
	}

	db := client.DB(context.TODO(), "devices")

	query := map[string]interface{}{
		"selector": map[string]interface{}{
			"type._id": map[string]interface{}{
				"$regex": "AtlonaGateway",
			},
		},
	}

	rows, err := db.Find(context.TODO(), query)
	if err != nil {
		log.L.Fatalf("couldn't get device")
	}

	var agwList []device
	for rows.Next() {
		var dev device
		if err := rows.ScanDoc(&dev); err != nil {
			log.L.Debugf("unable to scan in doc")
		}

		agwList = append(agwList, dev)
	}

	log.L.Debugf("Length of AGWlist: %d", len(agwList))

	conns = make(map[string]*websocket.Conn)
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
		conns[i.ID] = ws
	}

	//subscribe
	for i := range conns {
		err = conns[i].WriteMessage(websocket.BinaryMessage, []byte(`{"callBackId":4,"data":{"action":"SetCurrentPage","state":"{\"Page\":\"roomModifyDevices\"}","controller":"App"}}`))
		if err != nil {
			log.L.Debugf("unable to read message: %s", err)
		}

		go connection.ReadMessage(conns[i], i)
		go connection.SendKeepAlive(conns[i], i)
	}

	for {
		//wait some time
		log.L.Debugf("Waiting to check for list changes")
		time.Sleep(1 * time.Minute)

		log.L.Debugf("Checking AGWList for changes")

		rows, err := db.Find(context.TODO(), query)
		if err != nil {
			log.L.Fatalf("couldn't get device")
		}

		var newAGWList []device
		for rows.Next() {
			var dev device
			if err := rows.ScanDoc(&dev); err != nil {
				log.L.Debugf("unable to scan in doc")
			}

			newAGWList = append(newAGWList, dev)
		}

		//check to see if the length is different
		if len(conns) < len(newAGWList) {
			newList := make([]device, len(agwList))
			copy(newList, agwList)
			log.L.Debugf("comparing the list with the map to find the new one")

			for i := 0; i < len(newList); i++ {
				//for each object in newList check to see if it exists in the map of conns already
				new := newList[i]
				match := false

				for j := range conns {

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
					conns[new.ID] = ws
					//this is the message that tells the gateway to send device update events
					err = ws.WriteMessage(websocket.BinaryMessage, []byte(`{"callBackId":4,"data":{"action":"SetCurrentPage","state":"{\"Page\":\"roomModifyDevices\"}","controller":"App"}}`))
					if err != nil {
						log.L.Debugf("unable to read message: %s", err)
					}

					go connection.ReadMessage(ws, new.ID)
					go connection.SendKeepAlive(ws, new.ID)
				}
			}
		} else if len(conns) > len(newAGWList) {
			var found bool
			for i := range conns {
				found = false
				for _, x := range newAGWList {
					if i == x.ID {
						found = true
						break
					}
				}
				if found == false {
					delete(conns, i)
				}
			}
		}
	}
}
