package neose_mini

import (
	"errors"
	"log"
	"time"

	"go.bug.st/serial/enumerator"
	"gobot.io/x/gobot/platforms/firmata"
)

// https://prod-edam.honeywell.com/content/dam/honeywell-edam/sps/siot/en-us/products/sensors/humidity-with-temperature-sensors/common/documents/sps-siot-i2c-comms-humidicon-tn-009061-2-en-ciid-142171.pdf
const (
	HIH_I2C_ADDR                  int  = 0x27
	HIH_INIT_MEASURE_BLOCK_NUMBER byte = 0xa0
	HIH_STATUS_NORMAL             byte = 0
	HIH_STATUS_STALE              byte = 1
	HIH_STATUS_COMMAND_MODE       byte = 2
	HIH_STATUS_ERROR              byte = 3
)

const (
	GPIO_PIN_LED        = "4"
	GPIO_PIN_FAN        = "9"
	GPIO_PIN_LC_SHUTTER = "5"
	GPIO_PIN_PUMP       = "6"
)

type HIHValue struct {
	Status      byte
	Humidity    float64
	Temperature float64
}

type NeoseMini struct {
	PortName       string
	FirmataAdaptor *firmata.Adaptor
}

func NewNeoseMini() *NeoseMini {
	return &NeoseMini{}
}

func (neose *NeoseMini) Connect() error {
	var err error

	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return err
	}
	for _, port := range ports {
		if port.IsUSB {
			if port.VID == "2341" && port.PID == "8037" {
				neose.PortName = port.Name
			}
		}
	}
	if neose.PortName == "" {
		err = errors.New("neose mini board not connected")
		return err
	} else {
		log.Printf("Found Arduino Micro port: %s", neose.PortName)
		firmataAdaptor := firmata.NewAdaptor(neose.PortName)
		err = firmataAdaptor.Connect()
		if err != nil {
			return err
		}
		neose.FirmataAdaptor = firmataAdaptor
	}
	return err
}

func (neose *NeoseMini) Startup() {
	neose.LedOn()
	neose.LCShutterOn()
	neose.PumpSet(10)
}

func (neose *NeoseMini) Shutdown() {
	neose.LedOff()
	neose.LCShutterOff()
	neose.PumpSet(0)
}

func (neose *NeoseMini) Disconnect() {
	neose.FirmataAdaptor.Disconnect()
}

func (neose *NeoseMini) LedOn() error {
	return neose.FirmataAdaptor.DigitalWrite(GPIO_PIN_LED, 1)
}

func (neose *NeoseMini) LedOff() error {
	return neose.FirmataAdaptor.DigitalWrite(GPIO_PIN_LED, 0)
}

func (neose *NeoseMini) FanOn() error {
	return neose.FirmataAdaptor.DigitalWrite(GPIO_PIN_FAN, 1)
}

func (neose *NeoseMini) FanOff() error {
	return neose.FirmataAdaptor.DigitalWrite(GPIO_PIN_FAN, 0)
}

func (neose *NeoseMini) LCShutterOn() error {
	return neose.FirmataAdaptor.DigitalWrite(GPIO_PIN_LC_SHUTTER, 1)
}

func (neose *NeoseMini) LCShutterOff() error {
	return neose.FirmataAdaptor.DigitalWrite(GPIO_PIN_LC_SHUTTER, 0)
}

func (neose *NeoseMini) PumpSet(value byte) error {
	value &= 254
	return neose.FirmataAdaptor.PwmWrite(GPIO_PIN_PUMP, value)
}

func (neose *NeoseMini) HIHReadToChan(hihCh chan HIHValue, errCh chan error) {
	var err error
	var hihValue HIHValue

	i2c := firmata.NewFirmataI2cConnection(neose.FirmataAdaptor, HIH_I2C_ADDR)
	defer i2c.Close()

	// fmt.Printf("i2c: %#v\n", i2c)

	for {
		// Request a measure
		err = i2c.WriteBlockData(HIH_INIT_MEASURE_BLOCK_NUMBER, []byte{0})
		if err != nil {
			errCh <- err
			return
		}

		time.Sleep(time.Millisecond * 37) // Typical measurement cycle takes 36.65ms

		// Read measurement results
		var data []byte = make([]byte, 4)
		_, err := i2c.Read(data)
		if err != nil {
			log.Println(err)
			errCh <- err
			return
		}
		// log.Printf("I2C read %d bytes", n)
		// fmt.Printf("data: %v\n", data)
		hihValue.Status = data[0] >> 6 // Status is in first 2 bits

		// Convert bytes into uint16
		var intData []uint16 = make([]uint16, 4)
		for i, b := range data {
			intData[i] = uint16(b)
		}
		raw_humidity := ((intData[0] & 0b00111111) << 8) | intData[1] // first byte has only 6 signficant bits (2 first bits is status)
		hihValue.Humidity = float64(raw_humidity) / 16382             // 14 bits precision has 16383 levels

		raw_temperature := ((intData[2] << 8) | intData[3]) >> 2         // last 2 bits of last byte to be ignored
		hihValue.Temperature = (float64(raw_temperature)/16382)*165 - 40 // Honeywell calibration: gain=165, offset=-40

		hihCh <- hihValue
	}
}

func HIHConsumeChan(hihCh chan HIHValue, errCh chan error) {
	for hihValue := range hihCh {
		if hihValue.Status == HIH_STATUS_NORMAL {
			log.Printf("Humidity: %.2f; Temperature: %.2f [%d]", hihValue.Humidity, hihValue.Temperature, hihValue.Status)
		}
	}
}
