package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	// "os/exec"
	"encoding/xml"
	"io/ioutil"

	"github.com/aler9/gomavlib"
	"github.com/aler9/gomavlib/pkg/dialects/common"
	"github.com/aler9/gomavlib/pkg/msg"
	"github.com/influxdata/influxdb-client-go/v2"
)

const systemID byte = 255
const host string = "127.0.0.1"

type Mavlink struct {
	XMLName 	xml.Name 	`xml:"mavlink"`
	Messages	Messages	`xml:"messages"`
}

type Messages struct {
	XMLName		xml.Name	`xml:"messages"`
	Messages	[]Message	`xml:"message"`
}

type Message struct {
	XMLName xml.Name `xml:"message"`
	ID		string	 `xml:"id,attr"` 
	Fields 	[]Field	 `xml:"field"`
}

type Field struct {
	XMLName xml.Name `xml:"field"`
	Name string `xml:"name,attr"`
}


/* Takes a message and returns an array of strings where each element 
	is a message parameter*/
func parseValues(message msg.Message) []string {

	str := fmt.Sprintf("%v", message)
	str = str[2:len(str)-1]

	return strings.Fields(str)
}

/* Convert an array of string parameters to float so that InfluxDB can process them */
func convertToFloats(stringValues []string) []float64 {

	 floatValues := make([]float64, len(stringValues))

	for idx := range stringValues {
		floatVal, err := strconv.ParseFloat(stringValues[idx], 32) 
		if err != nil {
			panic(err)
		}
		floatValues[idx] = floatVal
	}

	return floatValues
}


func writeToInflux(id int, floatValues []float32) bool {

	return false
}

func main() {
	//maybe use godotenv for this
	const token = "7hK-vq0LuZFzXjqIQtCiAXD0BwLUWyAoen4mYhD2EO_NIcC5puPcMjjpy6syBpY9pWd6HO_JdBd2CgPMNIFoNw=="
	const bucket = "Mavlink"
	const org = "TritonUAS"

	client := influxdb2.NewClient("http://localhost:8086", token)
	writeAPI := client.WriteAPI(org, bucket)

	node, err := gomavlib.NewNode(gomavlib.NodeConf{
		Endpoints: []gomavlib.EndpointConf{
			gomavlib.EndpointTCPClient{fmt.Sprintf("%v:%v", host, "14551")},
			// gomavlib.EndpointUDPClient{fmt.Sprintf("%v:%v", host, "14550")},
		},
		Dialect:     common.Dialect,
		OutVersion:  gomavlib.V2, // change to V1 if you're unable to communicate with the target
		OutSystemID: systemID,
	})
	if err != nil {
		panic(err)
	}
	defer node.Close()

	mavXML, err := os.Open("common.xml")
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully Opened users.xml")
	defer mavXML.Close()
	byteValue, _ := ioutil.ReadAll(mavXML)
	// if err != nil {
	// 	panic(err)
	// }

	var mavlinkXML Mavlink

	xml.Unmarshal(byteValue, &mavlinkXML)



	for i := 0; i < len(mavlinkXML.Messages.Messages); i++ {
		id := mavlinkXML.Messages.Messages[i].ID
		if id == "74" {
			for j := 0; j < len(mavlinkXML.Messages.Messages[i].Fields); j++ {
				fmt.Println(mavlinkXML.Messages.Messages[i].Fields[j].Name)
			}
		}
	}


	for evt := range node.Events() {
		if frm, ok := evt.(*gomavlib.EventFrame); ok {
			msgID := frm.Message().GetID()

			//runs the findsIDs script which keeps track of all unique IDs
			// arg1 := fmt.Sprintf("%v", msgID)
			// cmd := exec.Command("./findIDs.bash", arg1)
			// out, err := cmd.Output()
			// if err != nil {
			// 	panic("FINDIDS error")
			// }
			// fmt.Println(string(out))
			
			values := parseValues(frm.Message())

			switch msgID {

			case 1:
			case 22:
			case 24:
			case 27:
			case 29:
			case 30:
			case 32:
			case 33:
			case 35:
			case 36:
			case 42:
			case 65:
			case 74:		
				floatValues := convertToFloats(values)

				//make a fucntion taht takes the ID and finds the names of the parameters
				p := influxdb2.NewPointWithMeasurement("VFR_HUD").
					AddTag("ID", "74").
					AddField("airspeed", floatValues[0]).
					AddField("groundspeed", floatValues[1]).
					AddField("heading", floatValues[2]).
					AddField("throttle", floatValues[3]).
					AddField("alt", floatValues[4]).
					AddField("climb", floatValues[5]).
					SetTime(time.Now())

				writeAPI.WritePoint(p)
				writeAPI.Flush()
				// fmt.Printf("Wrote %v to influxDB \n", values)
			case 116:
			case 125:
			case 136:
			case 147:
			case 150:
			case 152:
			case 163:
			case 164:
			case 165:
			case 168:
			case 178:
			case 182:
			case 193:
			case 241:
			case 242:
				
			

			
			}
			
			
		}
	}

	defer client.Close()


}

