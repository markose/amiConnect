Description:
==============
The Adapter can be used to connect to the AMI interface of an (remote) Asterisk instance.

Constructor:
NewAMIAdapter(ip string) (*AMIAdapter, error)


Methods:

Login(username string, password string) (chan map[string]string, error)
Returns a channel on which key-value based AMI events can be received 

Exec(action map[string]string) map[string]string
Executes an action and returns the key-value based response from the server

Example Usage:
==============
	package main

	import (
		"github.com/markose/amiConnect"
		"fmt"
		"log"
		"sync"
	)
	
	func main() {
	
		var err error
		var a *amiConnect.AMIAdapter
		var events chan map[string]string
	
		a, err = amiConnect.NewAMIAdapter("127.0.0.1")
		if err != nil {
			log.Fatalln(err)
		}
	
		events, err = a.Login("testuser", "testsecret")
		if err != nil {
			log.Fatalln(err)
		}
	
		go func() {
			for {
				var event = <-events
	
				log.Println("---EVENT---")
				for e := range event {
					log.Println(e + ":" + event[e])
				}
				log.Println("------------")
			}
		}()
	
		var action = map[string]string{
			"Action":   "Setvar",
			"ActionID": "Setvar_1234",
			"Channel":  "SOME_CHANNEL",
			"Variable": "SOME_VAR",
			"Value":    "SOME_VALUE",
		}
	
		result := a.Exec(action)
	
		if result["Response"] == "Success" {
			fmt.Printf("SUCCESS: Set variable")
		} else if result["Response"] == "Error" {
			fmt.Printf("ERROR: Set variable")
		}
	
		var w sync.WaitGroup
		w.Add(1)
		w.Wait()
	}