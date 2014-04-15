package amiConnect

import (
	"bufio"
	"bytes"
	"errors"
	"log"
	"net"
	"strconv"
	"time"
)

type AMIAdapter struct {
	ip       string
	username string
	password string

	chanActions   chan map[string]string
	chanResponses chan map[string]string
	chanEvents    chan map[string]string
}

func NewAMIAdapter(ip string) (*AMIAdapter, error) {

	var a = new(AMIAdapter)
	a.ip = ip

	conn, err := openConnection(ip)
	if err != nil {
		return nil, err
	}

	chanOutStreamReader := make(chan byte)
	a.chanActions = make(chan map[string]string)

	chanQuitActionWriter := actionWriter(conn, a.chanActions)
	chanErrStreamReader := streamReader(conn, chanOutStreamReader)
	chanOutStreamParser := streamParser(chanOutStreamReader)
	a.chanResponses, a.chanEvents = classifier(chanOutStreamParser)

	go func() {
		for {
			err := <-chanErrStreamReader
			chanQuitActionWriter <- true

			log.Println("TCP ERROR")

			for i := 100; i >= 0; i-- {

				if i == 0 {
					log.Fatalln("Reconnect failed 100 times. Give up!")
				}

				log.Println("Try reconnect in 10 seconds")
				time.Sleep(time.Second * 10)

				conn, err = openConnection(ip)
				if err != nil {
					log.Println("Reconnect failed! Retries remaining: " + strconv.Itoa(i))
				} else {
					chanErrStreamReader = streamReader(conn, chanOutStreamReader)
					chanQuitActionWriter = actionWriter(conn, a.chanActions)

					_, err = a.Login(a.username, a.password)
					if err != nil {
						log.Fatalln("Login failed!")
					}
					break
				}
			}
		}
	}()

	return a, nil
}

func (a *AMIAdapter) Login(username string, password string) (chan map[string]string, error) {

	a.username = username
	a.password = password

	var action = map[string]string{
		"Action":   "Login",
		"Username": a.username,
		"Secret":   a.password,
	}

	var result = a.Exec(action)
	if result["Response"] == "Error" {
		return nil, errors.New("Login failed: " + result["Message"])
	}

	return a.chanEvents, nil
}

func (a *AMIAdapter) Exec(action map[string]string) map[string]string {

	a.chanActions <- action
	var response = <-a.chanResponses
	return response
}

func streamReader(conn *net.TCPConn, chanOut chan byte) (chanErr chan error) {

	chanErr = make(chan error)

	reader := bufio.NewReader(conn)

	go func() {
		for {
			b, err := reader.ReadByte()
			if err != nil {
				chanErr <- err
				return
			}
			chanOut <- b
		}
	}()

	return chanErr
}

func actionWriter(conn *net.TCPConn, in chan map[string]string) (chanQuit chan bool) {

	chanQuit = make(chan bool)

	go func() {
		for {
			select {
			case action := <-in:
				{
					var data = serialize(action)
					_, err := conn.Write(data)
					if err != nil {
						return
					}
				}
			case <-chanQuit:
				{
					return
				}
			}
		}
	}()

	return chanQuit
}

func streamParser(in chan byte) (chanOut chan map[string]string) {

	chanOut = make(chan map[string]string)

	var data = make(map[string]string)
	var wordBuf bytes.Buffer
	var key string
	var value string
	var lastByte byte
	var curByte byte
	var state = 0 // 0: key state, 1: value state

	go func() {

		for {
			lastByte = curByte
			curByte = <-in

			if curByte == ':' || curByte == '\n' {
				continue
			}

			switch state {
			case 0:
				{
					if curByte == ' ' {
						if lastByte == ':' {
							key = wordBuf.String()
							wordBuf.Reset()
							state = 1
						}
					} else if curByte == '\r' {
						if len(value) > 0 {
							chanOut <- data
							data = make(map[string]string)
						}
						wordBuf.Reset()
						key = ""
						value = ""
						lastByte = 0
						curByte = 0
						state = 0
					} else {
						wordBuf.WriteByte(curByte)
					}
				}
			case 1:
				{
					if curByte == '\r' {
						value = wordBuf.String()
						wordBuf.Reset()
						state = 0
						data[key] = value
					} else {
						wordBuf.WriteByte(curByte)
					}
				}
			}
		}
	}()

	return chanOut
}

func classifier(in chan map[string]string) (chanOutResponses chan map[string]string, chanOutEvents chan map[string]string) {

	chanOutResponses = make(chan map[string]string)
	chanOutEvents = make(chan map[string]string)

	go func() {
		for {
			data := <-in

			for d := range data {
				switch d {
				case "Response":
					chanOutResponses <- data
					break
				case "Event":
					chanOutEvents <- data
					break
				}
			}
		}
	}()

	return chanOutResponses, chanOutEvents
}

func openConnection(ip string) (*net.TCPConn, error) {

	socket := ip + ":5038"

	raddr, err := net.ResolveTCPAddr("tcp", socket)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialTCP("tcp", nil, raddr)
	if err != nil {
		return nil, err
	}

	return conn, err
}

func serialize(data map[string]string) []byte {

	var outBuf bytes.Buffer

	for key := range data {
		value := data[key]

		outBuf.WriteString(key)
		outBuf.WriteString(": ")
		outBuf.WriteString(value)
		outBuf.WriteString("\n")
	}
	outBuf.WriteString("\n")
	return outBuf.Bytes()
}
