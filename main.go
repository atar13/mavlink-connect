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
	"github.com/influxdata/influxdb-client-go/v2/api"
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
	MsgName	string	 `xml:"name,attr"` 
	Fields 	[]Field	 `xml:"field"`
}

type Field struct {
	XMLName xml.Name `xml:"field"`
	Name string `xml:"name,attr"`
}

type entry struct {
	value float64 
} 


/* Takes a message and returns an array of strings where each element 
	is a message parameter*/
func parseValues(message msg.Message) []string {

	str := fmt.Sprintf("%v", message)
	str = str[2:len(str)-1]

	return strings.Fields(str)
}

/* Convert an array of string parameters to float so that InfluxDB can process them */
func convertToFloats(stringValues []string, tmp uint32) []float64 {

	floatValues := make([]float64, len(stringValues))

	for idx := range stringValues {
		floatVal, err := strconv.ParseFloat(stringValues[idx], 32) 
		if err != nil {
			fmt.Println(tmp)
			// panic(err)
		}
		floatValues[idx] = floatVal
	}

	return floatValues
}

func getParameterNames(msgID uint32, mavlink Mavlink)([]string, string) {
	
	var parameterNames []string
	var msgName string

	for i := 0; i < len(mavlink.Messages.Messages); i++ {
		id := mavlink.Messages.Messages[i].ID
		msgName = mavlink.Messages.Messages[i].MsgName
		intID, err := strconv.ParseInt(id, 10, 32)
		if err != nil {
			panic(err)
		}
		if intID == int64(msgID) {

			//TODO: improve this search algorithm
			for j := 0; j < len(mavlink.Messages.Messages[i].Fields); j++ {
				parameterNames = append(parameterNames, mavlink.Messages.Messages[i].Fields[j].Name)
			}
			break
		}
	}
	return parameterNames, msgName
}

//write the data of a particular message to the local influxDB 
func writeToInflux(msgID uint32, msgName string, parameters []string, floatValues []float64, writeAPI api.WriteAPI) {

	for idx := range parameters {
		p := influxdb2.NewPointWithMeasurement(msgName).
		AddTag("ID", fmt.Sprintf("%v", msgID)).
		AddField(parameters[idx], floatValues[idx]).
		SetTime(time.Now())
		writeAPI.WritePoint(p)
	}

	// p := influxdb2.NewPoint("VFR_HUD",
	// 	map[string]string{"ID": fmt.Sprintf("%v", msgID)},
	// 	map[string]interface{},
	// 	time.Now())


	writeAPI.Flush()
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
	byteValue, err := ioutil.ReadAll(mavXML)
	if err != nil {
		panic(err)
	}

	var mavlink Mavlink

	xml.Unmarshal(byteValue, &mavlink)


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
			
			rawValues := parseValues(frm.Message())

			switch msgID {

			case 1:
				fallthrough
			
			//error with parsing 24
			// case 24:
			// 	fallthrough
			case 27:
				fallthrough
			case 29:
				fallthrough
			case 30:
				fallthrough
			case 32:
				fallthrough
			case 33:
				fallthrough
			case 35:
				fallthrough
			case 36:
				fallthrough
			case 40:
				fallthrough
			case 42:
				fallthrough
			case 46:
				fallthrough
			case 62:
				fallthrough
			case 65:
				fallthrough
			case 74:		
				fallthrough
			case 77:
				//error with 77
				fallthrough
			
			//error with parsing 87
			// case 87:
			// 	fallthrough
			case 116:
				fallthrough
			case 125:
				fallthrough
			case 136:
				fallthrough
			case 241:
				floatValues := convertToFloats(rawValues, msgID)
				parameters, msgName := getParameterNames(msgID, mavlink)
				// fmt.Println(parameters)
				writeToInflux(msgID, msgName, parameters, floatValues, writeAPI)


			//Messages below don't work with all floats and need custom parsing
			case 22:
				//1st element is a char[]
				//maybe use struct?
				type PARAM_VALUE struct {
					param_id 	[16]rune
					param_value float64
					param_type	uint8
					param_count uint16
					param_index uint16
				}
				parameters, msgName := getParameterNames(msgID, mavlink)
				// fmt.Println(parameters, msgName)
				fmt.Println(rawValues)
				p := influxdb2.NewPointWithMeasurement(msgName).
				AddTag("ID", fmt.Sprintf("%v", msgID)).
				AddField(parameters[0], rawValues[0]).
				SetTime(time.Now())
				writeAPI.WritePoint(p)
					
				floatValues := convertToFloats(rawValues[1:2], msgID)
				floatValues = append(floatValues, convertToFloats(rawValues[3:], msgID)...)
				for i := 1; i < len(parameters); i++ {
					if i == 2 {
						break
					}
					p := influxdb2.NewPointWithMeasurement(msgName).
					AddTag("ID", fmt.Sprintf("%v", msgID)).
					AddField(parameters[i], floatValues[i-1]).
					SetTime(time.Now())
					writeAPI.WritePoint(p)
				}

				writeAPI.Flush()


			case 147:
				//has two arrays of integers
			case 242:
				//one array
			case 253:
				//array of chars

			//Messages below aren't documented 
			//TODO: Look into what they are 
			case 150:
			case 152:
			case 163:
			case 164:
			case 165:
			case 168:
			case 174:
			case 178:
			case 182:
			case 193:

			}
		}
	}
	defer client.Close()
}

