package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"go.neose-mini.firmata-client/neose_mini"
)

func main() {
	var err error

	neose := neose_mini.NewNeoseMini()
	err = neose.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer neose.Disconnect()

	// neose.Startup()

	hihCh := make(chan neose_mini.HIHValue, 1)
	errCh := make(chan error, 1)

	go neose.HIHReadToChan(hihCh, errCh)
	// go neose_mini.HIHConsumeChan(hihCh, errCh)

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter <device>:<value>: ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		log.Printf("You entered %s (%d)", text, len(text))

		parts := strings.Split(text, ":")
		if len(parts) < 2 {
			continue
		}
		dev := parts[0]
		val := parts[1]
		switch dev {
		case "FAN":
			switch val {
			case "0":
				neose.FanOff()
			case "1":
				neose.FanOn()
			}
		case "LCS":
			switch val {
			case "0":
				neose.LCShutterOff()
			case "1":
				neose.LCShutterOn()
			}
		case "LED":
			switch val {
			case "0":
				neose.LedOff()
			case "1":
				neose.LedOn()
			}
		case "PMP":
			iVal, err := strconv.Atoi(val)
			if err != nil {
				log.Println(err)
				break
			}
			bVal := byte(iVal & 0xff)
			neose.PumpSet(bVal)
		default:
			log.Print("Did not recognize, try again!")
		}
		hihValue := <-hihCh
		log.Printf("Humidity: %.2f; Temperature: %.2f [%d]", hihValue.Humidity, hihValue.Temperature, hihValue.Status)
	}
}
