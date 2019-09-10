package main

import (
	"net/http"
	"os"

	"github.com/byuoitav/atlona-event-ms/connection"
	"github.com/byuoitav/common"
	"github.com/gorilla/websocket"

	"github.com/byuoitav/common/db/couch"
	"github.com/byuoitav/common/log"
)

var (
	address  = os.Getenv("DB_ADDRESS")
	username = os.Getenv("DB_USERNAME")
	password = os.Getenv("DB_PASSWORD")
)

func init() {
	if len(address) == 0 || len(username) == 0 || len(password) == 0 {
		log.L.Fatalf("One of DB_ADDRESS, DB_USERNAME, DB_PASSWORD is not set. Failing...")
	}
}

func main() {
	log.SetLevel("debug")

	router := common.NewRouter()

	port := ":10000"

	db := couch.NewDB(address, username, password)
	agwList, err := db.GetDevicesByType("AtlonaGateway")
	log.L.Debugf("This is the length of agwlist: %d", len(agwList))

	if err != nil {
		log.L.Fatalf("There was an error getting the AGWList: %v", err)
	}

	go connection.MonitorAGWList(agwList)

	//subscribe
	for i := range connection.Conns {
		err = connection.Conns[i].WriteMessage(websocket.BinaryMessage, []byte(`{"callBackId":4,"data":{"action":"SetCurrentPage","state":"{\"Page\":\"roomModifyDevices\"}","controller":"App"}}`))
		if err != nil {
			log.L.Debugf("unable to read message: %s", err)
		}

		go connection.ReadMessage(connection.Conns[i])
	}

	server := http.Server{
		Addr:           port,
		MaxHeaderBytes: 1024 * 10,
	}
	router.StartServer(&server)
}
