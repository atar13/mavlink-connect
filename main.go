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
	"github.com/aler9/gomavlib/pkg/dialects/ardupilotmega"
	"github.com/aler9/gomavlib/pkg/msg"
	"github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

const systemID byte = 255
const host string = "127.0.0.1"

type Mavlink struct {
	XMLName 	xml.Name 	`xml:"mavlink"`
	Enums		Enums		`xml:"enums"`
	Messages	Messages	`xml:"messages"`
}

type Enums struct {
	XMLName 	xml.Name 	`xml:"enums"`
	Enums 		[]Enum 		`xml:"enum"`
}

type Enum struct {
	XMLName 	xml.Name	`xml:"enum"`
	Entries		[]Entry		`xml:"entry"`
	Name 		string 		`xml:"name,attr"`
}

type Entry struct {
	XMLName 	xml.Name 	`xml:"entry"`
	Value 		string 		`xml:"value,attr"`
	Name 		string 		`xml:"name,attr"`
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
	Name 	string 	 `xml:"name,attr"`
	Enum	string 	 `xml:"enum,attr"` 
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
			
			fmt.Println(idx, "Message ID", tmp, "is causing a float parsing error.")
			// panic(err)
		}
		floatValues[idx] = floatVal
	}

	return floatValues
}

//retreive mavlink message paramters based on the message ID and type of .xml file to look in
func getParameterNames(msgID uint32, mavlink Mavlink)([]string, string) {
	var parameterNames []string
	var msgName string

	//TODO: improve this search algorithm
	for i := 0; i < len(mavlink.Messages.Messages); i++ {
		id := mavlink.Messages.Messages[i].ID
		msgName = mavlink.Messages.Messages[i].MsgName
		intID, err := strconv.ParseInt(id, 10, 32)
		if err != nil {
			panic(err)
		}
		if intID == int64(msgID) {
			for j := 0; j < len(mavlink.Messages.Messages[i].Fields); j++ {
				parameterNames = append(parameterNames, mavlink.Messages.Messages[i].Fields[j].Name)
			}
			break
		}
	}
	return parameterNames, msgName
}

//Retrives the name of an enum type based on message ID and the index the enum appears in a field
func getEnumTypeFromField(msgID uint32, fieldIndex int, mavlink Mavlink) string {

	for i := 0; i < len(mavlink.Messages.Messages); i++ {
		id := mavlink.Messages.Messages[i].ID
		intID, err := strconv.ParseInt(id, 10, 32)
		if err != nil {
			panic(err)
		}
		if(intID == int64(msgID)){
			enumType := mavlink.Messages.Messages[i].Fields[fieldIndex].Enum
			return enumType
		}
	}
	return ""
}

//Retrive the integer representation of an enum string representation
func getIntFromEnum(msgID uint32, fieldIndex int, enumVal string, mavlink Mavlink) uint {
	/**
	look up msgID and get field
	go to index of the enum,
	get the name of the enum
	look up enum,
	find an enum entry with the same as the rawValue from the plane
	get the value of that entry and return it
	**/

	enumType := getEnumTypeFromField(msgID, fieldIndex, mavlink)

	for i := 0; i < len(mavlink.Enums.Enums); i++ {
		if mavlink.Enums.Enums[i].Name == enumType {
			for j := 0; j < len(mavlink.Enums.Enums[i].Entries); j++ {
				if mavlink.Enums.Enums[i].Entries[j].Name == enumVal {
					stringValue := mavlink.Enums.Enums[i].Entries[j].Value 
					value, err := strconv.ParseUint(stringValue, 10, 32);
					if err != nil {
						return 999
					}
					return uint(value)
				}
			}
		}
	}
	return 999
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
	writeAPI.Flush()
}

func main() {
	//maybe use godotenv for this
	const token = "-0CJSHCejCNNlgEi-0MhuWahkmNSm5GzuPCw8scyvjZNhIDYCux93ljSXoTGNbWl4-eWThnDxIYU78z082152w=="
	const bucket = "Mavlink"
	const org = "TritonUAS"

	client := influxdb2.NewClient("http://localhost:8086", token)
	writeAPI := client.WriteAPI(org, bucket)

	node, err := gomavlib.NewNode(gomavlib.NodeConf{
		Endpoints: []gomavlib.EndpointConf{
			gomavlib.EndpointTCPClient{fmt.Sprintf("%v:%v", host, "14551")},
			// gomavlib.EndpointUDPClient{fmt.Sprintf("%v:%v", host, "14550")},
		},
		Dialect:     ardupilotmega.Dialect,
		OutVersion:  gomavlib.V2, // change to V1 if you're unable to communicate with the target
		OutSystemID: systemID,
	})
	if err != nil {
		panic(err)
	}
	defer node.Close()




	mavXML, err := os.Open("common.xml")
	arduXML, err := os.Open("ardupilotmega.xml")
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully Opened common.xml and ardupilotmega.xml")
	defer mavXML.Close()
	defer arduXML.Close()
	mavByteValue, err := ioutil.ReadAll(mavXML)
	arduPilotByteValue, err := ioutil.ReadAll(arduXML)
	if err != nil {
		panic(err)
	}

	var mavlinkCommon Mavlink
	var arduPilotMega Mavlink

	xml.Unmarshal(mavByteValue, &mavlinkCommon)
	xml.Unmarshal(arduPilotByteValue, &arduPilotMega)


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

			//normal cases with no arrays or enums
			case 1:
				fallthrough
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
			case 116:
				fallthrough
			case 125:
				fallthrough
			case 136:
				fallthrough
			case 241:
				floatValues := convertToFloats(rawValues, msgID)
				parameters, msgName := getParameterNames(msgID, mavlinkCommon)
				writeToInflux(msgID, msgName, parameters, floatValues, writeAPI)


			//Messages below don't work with all floats and need custom parsing
			case 22:
				parameters, msgName := getParameterNames(msgID, mavlinkCommon)

				//TODO: add param_type enum
				p := influxdb2.NewPointWithMeasurement(msgName).
				AddTag("ID", fmt.Sprintf("%v", msgID)).
				AddField(parameters[0], rawValues[0]).
				SetTime(time.Now())
				writeAPI.WritePoint(p)
					
				floatValues := convertToFloats(rawValues[1:2], msgID)
				floatValues = append(floatValues, convertToFloats(rawValues[3:], msgID)...)


				floatParameters := parameters[1:2]
				floatParameters = append(floatParameters, parameters[3:]...)
				writeToInflux(msgID, msgName, floatParameters, floatValues, writeAPI)

							
			// error with parsing 24
			case 24:
				//one enum value
				parameters, msgName := getParameterNames(msgID, mavlinkCommon)


				fix_type := float64(getIntFromEnum(msgID, 1, rawValues[1], mavlinkCommon))
				enumVals := []float64{fix_type}

				enumNames := []string{parameters[1]}

				writeToInflux(msgID, msgName, enumNames, enumVals, writeAPI)


				floatValues := convertToFloats(rawValues[0:1], msgID)
				floatValues = append(floatValues, convertToFloats(rawValues[2:], msgID)...)

				
				floatParameters := parameters[0:1]
				floatParameters = append(floatParameters, parameters[2:]...)
				writeToInflux(msgID, msgName, floatParameters, floatValues, writeAPI)

			case 77:
				//2 enum values
				parameters, msgName := getParameterNames(msgID, mavlinkCommon)

				command := float64(getIntFromEnum(msgID, 0, rawValues[0], mavlinkCommon))
				result := float64(getIntFromEnum(msgID, 1, rawValues[1], mavlinkCommon))
				enumVals := []float64{command, result}

				var enumNames []string
				enumNames = append(enumNames, parameters[0:2]...)

				writeToInflux(msgID, msgName, enumNames, enumVals, writeAPI)

				floatValues := convertToFloats(rawValues[2:], msgID)

				
				floatParameters := parameters[2:]
				writeToInflux(msgID, msgName, floatParameters, floatValues, writeAPI)

				// error with parsing 87
			case 87:
				//two enum values
				parameters, msgName := getParameterNames(msgID, mavlinkCommon)

				coordinate_frame := float64(getIntFromEnum(msgID, 1, rawValues[1], mavlinkCommon))
				type_mask := float64(getIntFromEnum(msgID, 2, rawValues[2], mavlinkCommon))
				enumVals := []float64{coordinate_frame, type_mask}

				var enumNames []string
				enumNames = append(enumNames, parameters[1:3]...)

				writeToInflux(msgID, msgName, enumNames, enumVals, writeAPI)


				floatValues := convertToFloats(rawValues[0:1], msgID)
				floatValues = append(floatValues, convertToFloats(rawValues[3:], msgID)...)

				floatParameters := parameters[0:1]
				floatParameters = append(floatParameters, parameters[3:]...)
				writeToInflux(msgID, msgName, floatParameters, floatValues, writeAPI)
				
			case 147:
				parameters, msgName := getParameterNames(msgID, mavlinkCommon)

				//TODO: handle enum cases for battery status
				// fmt.Printf("%v",rawValues[1])


				//parses array of battery voltage information for cells 1 to 10 
				voltageStrings := rawValues[4:14]
				for i := 0; i < len(voltageStrings); i++ {
					label := fmt.Sprintf("voltages%v", i)
					if i == 0 {
						voltageStrings[i] = (voltageStrings[i])[1:]
					} else if i == len(voltageStrings) -1 {
						voltageStrings[i] = (voltageStrings[i])[:len(voltageStrings[i])-1]
					} 
					value, err := strconv.ParseFloat(voltageStrings[i], 32)
					if err != nil {
						fmt.Println("Error with parsing message 147")
						break
					}
					p := influxdb2.NewPointWithMeasurement(msgName).
					AddTag("ID", fmt.Sprintf("%v", msgID)).
					AddField(label, value).
					SetTime(time.Now())
					writeAPI.WritePoint(p)
				}


				//parses array of battery voltage information for cells 11 to 14 
				voltageExtStrings := rawValues[20:24]
				for i := 0; i < len(voltageExtStrings); i++ {
					label := fmt.Sprintf("voltages_ext%v", i)
					if i == 0 {
						voltageExtStrings[i] = (voltageExtStrings[i])[1:]
					} else if i == len(voltageExtStrings) -1 {
						voltageExtStrings[i] = (voltageExtStrings[i])[:len(voltageExtStrings[i])-1]
					} 
					value, err := strconv.ParseFloat(voltageExtStrings[i], 32)
					if err != nil {
						fmt.Println("Error with parsing message 147")
						break
					}
					p := influxdb2.NewPointWithMeasurement(msgName).
					AddTag("ID", fmt.Sprintf("%v", msgID)).
					AddField(label, value).
					SetTime(time.Now())
					writeAPI.WritePoint(p)
				}

				
				//parse the rest of the values normally
				floatValues := convertToFloats(rawValues[0:1], msgID)
				floatValues = append(floatValues, convertToFloats(rawValues[3:4], msgID)...)
				floatValues = append(floatValues, convertToFloats(rawValues[14:19], msgID)...)
				floatValues = append(floatValues, convertToFloats(rawValues[25:], msgID)...)
				
				floatParameters := parameters[0:1]
				floatParameters = append(floatParameters, parameters[3:4]...)
				floatParameters = append(floatParameters, parameters[5:10]...)
				floatParameters = append(floatParameters, parameters[11:12]...)
				writeToInflux(msgID, msgName, floatParameters, floatValues, writeAPI)

				// writeAPI.Flush()
			case 242:
				parameters, msgName := getParameterNames(msgID, mavlinkCommon)

				fmt.Println(msgID)
				//one array
				floatValues := convertToFloats(rawValues[0:6], msgID)
				floatValues = append(floatValues, convertToFloats(rawValues[10:], msgID)...)

				
				floatParameters := parameters[0:6]
				floatParameters = append(floatParameters, parameters[7:]...)
				writeToInflux(msgID, msgName, floatParameters, floatValues, writeAPI)
			case 253:
				//array of chars
				//enum at index 0

				//TODO figure out what do to for status text
				parameters, msgName := getParameterNames(msgID, mavlinkCommon)

				fmt.Println("Fdsafdsfasdf")
				fmt.Println(rawValues)
				for i := 0; i < len(rawValues); i++ {
					fmt.Println(rawValues[i])
				}
				//one array

				floatValues := convertToFloats(rawValues[len(rawValues)-2:], msgID)
				floatParameters := parameters[len(parameters)-2:]
				writeToInflux(msgID, msgName, floatParameters, floatValues, writeAPI)



	
			//ardupilot dialectmessages 
			case 150:
				fallthrough
			case 152:
				fallthrough
			case 163:
				fallthrough
			case 164:
				fallthrough
			case 165:
				fallthrough
			case 168:
				fallthrough
			case 174:
				fallthrough
			case 178:
				fallthrough
			case 182:
				fallthrough
			case 193:
				floatValues := convertToFloats(rawValues, msgID)
				parameters, msgName := getParameterNames(msgID, arduPilotMega)
				writeToInflux(msgID, msgName, parameters, floatValues, writeAPI)

			}
		}
	}
	defer client.Close()
}

