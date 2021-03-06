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

//XML structs
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
func getIntValFromEnum(msgID uint32, fieldIndex int, enumVal string, mavlink Mavlink) uint {

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

	//InfluxDB credentials
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
		Dialect:     ardupilotmega.Dialect,
		OutVersion:  gomavlib.V2, // change to V1 if you're unable to communicate with the target
		OutSystemID: systemID,
	})
	if err != nil {
		panic(err)
	}
	defer node.Close()

	//read xml files of messages
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


	//loop through incoming events
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
			
			//gather the raw values returned by the plane as an array of strings
			rawValues := parseValues(frm.Message())

			switch msgID {

			//normal cases with no arrays or enums

			//SYS_STATUS
			case 1:
				fallthrough

			// RAW_IMU
			case 27:
				fallthrough

			// SCALED_PRESSURE
			case 29:
				fallthrough

			// ATTITUDE
			case 30:
				fallthrough

			// LOCAL_POSITION_NED
			case 32:
				fallthrough

			//GLOBAL_POSITION_INT
			case 33:
				fallthrough

			//RC_CHANNELS_RAW
			case 35:
				fallthrough

			//SERVO_OUTPUT_RAW
			case 36:
				fallthrough

			//MISSION_CURRENT
			case 42:
				fallthrough

			//MISSION_ITEM_REACHED
			case 46:
				fallthrough

			//NAV_CONTROLLER_OUTPUT
			case 62:
				fallthrough

			//RC_CHANNELS
			case 65:
				fallthrough

			//VFR_HUD
			case 74:		
				fallthrough

			//SCALED_IMU2
			case 116:
				fallthrough
			
			//POWER_STATUS
			case 125:
				fallthrough

			//TERRAIN_REPORT
			case 136:
				fallthrough

			// VIBRATION
			case 241:
				floatValues := convertToFloats(rawValues, msgID)
				parameters, msgName := getParameterNames(msgID, mavlinkCommon)
				writeToInflux(msgID, msgName, parameters, floatValues, writeAPI)


			//Messages below don't work with all floats and require custom parsing

			//PARAM_VALUE
			case 22:
				parameters, msgName := getParameterNames(msgID, mavlinkCommon)

				//enum parser
				paramType := float64(getIntValFromEnum(msgID, 2, rawValues[2], mavlinkCommon))
				enumVals := []float64{paramType}
				var enumNames []string
				enumNames = append(enumNames, parameters[2:3]...)
				writeToInflux(msgID, msgName, enumNames, enumVals, writeAPI)

				//remaining float parsing
				floatValues := convertToFloats(rawValues[1:2], msgID)
				floatValues = append(floatValues, convertToFloats(rawValues[3:], msgID)...)
				floatParameters := parameters[1:2]
				floatParameters = append(floatParameters, parameters[3:]...)
				writeToInflux(msgID, msgName, floatParameters, floatValues, writeAPI)

			//GPS_RAW_INT
			case 24:
				parameters, msgName := getParameterNames(msgID, mavlinkCommon)

				//enum parser
				fixType := float64(getIntValFromEnum(msgID, 1, rawValues[1], mavlinkCommon))
				enumVals := []float64{fixType}
				enumNames := []string{parameters[1]}
				writeToInflux(msgID, msgName, enumNames, enumVals, writeAPI)

				//remaining float parser
				floatValues := convertToFloats(rawValues[0:1], msgID)
				floatValues = append(floatValues, convertToFloats(rawValues[2:], msgID)...)
				floatParameters := parameters[0:1]
				floatParameters = append(floatParameters, parameters[2:]...)
				writeToInflux(msgID, msgName, floatParameters, floatValues, writeAPI)

			//MISSION_REQUEST
			case 40:
				parameters, msgName := getParameterNames(msgID, mavlinkCommon)

				//enum parser
				missionType := float64(getIntValFromEnum(msgID, 3, rawValues[3], mavlinkCommon))
				enumVals := []float64{missionType}
				enumNames := []string{parameters[3]}
				writeToInflux(msgID, msgName, enumNames, enumVals, writeAPI)

			//COMMAND_ACK
			case 77:
				parameters, msgName := getParameterNames(msgID, mavlinkCommon)

				//enum parser
				command := float64(getIntValFromEnum(msgID, 0, rawValues[0], mavlinkCommon))
				result := float64(getIntValFromEnum(msgID, 1, rawValues[1], mavlinkCommon))
				enumVals := []float64{command, result}
				var enumNames []string
				enumNames = append(enumNames, parameters[0:2]...)
				writeToInflux(msgID, msgName, enumNames, enumVals, writeAPI)

				//remaining float parser
				floatValues := convertToFloats(rawValues[2:], msgID)
				floatParameters := parameters[2:]
				writeToInflux(msgID, msgName, floatParameters, floatValues, writeAPI)

			//POSITION_TARGET_GLOBAL_INT
			case 87:
				//two enum values
				parameters, msgName := getParameterNames(msgID, mavlinkCommon)

				//enum parse and write 
				coordinateFrame := float64(getIntValFromEnum(msgID, 1, rawValues[1], mavlinkCommon))
				typeMask := float64(getIntValFromEnum(msgID, 2, rawValues[2], mavlinkCommon))
				enumVals := []float64{coordinateFrame, typeMask}
				var enumNames []string
				enumNames = append(enumNames, parameters[1:3]...)
				writeToInflux(msgID, msgName, enumNames, enumVals, writeAPI)


				floatValues := convertToFloats(rawValues[0:1], msgID)
				floatValues = append(floatValues, convertToFloats(rawValues[3:], msgID)...)

				floatParameters := parameters[0:1]
				floatParameters = append(floatParameters, parameters[3:]...)
				writeToInflux(msgID, msgName, floatParameters, floatValues, writeAPI)
				
			//BATTERY_STATUS
			case 147:
				parameters, msgName := getParameterNames(msgID, mavlinkCommon)

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

				//parse the remaining enum values
				batteryFunction := float64(getIntValFromEnum(msgID, 1, rawValues[1], mavlinkCommon))
				batteryType := float64(getIntValFromEnum(msgID, 2, rawValues[2], mavlinkCommon))
				chargingState := float64(getIntValFromEnum(msgID, 10, rawValues[19], mavlinkCommon))
				batteryMode := float64(getIntValFromEnum(msgID, 12, rawValues[24], mavlinkCommon))
				faultBitmask := float64(getIntValFromEnum(msgID, 13, rawValues[25], mavlinkCommon))
				enumVals := []float64{batteryFunction, batteryType, chargingState, batteryMode, faultBitmask}
				var enumNames []string
				enumNames = append(enumNames, parameters[1:3]...)
				enumNames = append(enumNames, parameters[10:11]...)
				enumNames = append(enumNames, parameters[12:]...)
				writeToInflux(msgID, msgName, enumNames, enumVals, writeAPI)
				
				//parse the rest of the values normally
				floatValues := convertToFloats(rawValues[0:1], msgID)
				floatValues = append(floatValues, convertToFloats(rawValues[3:4], msgID)...)
				floatValues = append(floatValues, convertToFloats(rawValues[14:19], msgID)...)
				// floatValues = append(floatValues, convertToFloats(rawValues[23:24], msgID)...)

				
				floatParameters := parameters[0:1]
				floatParameters = append(floatParameters, parameters[3:4]...)
				floatParameters = append(floatParameters, parameters[5:10]...)
				// floatParameters = append(floatParameters, parameters[11:12]...)
				writeToInflux(msgID, msgName, floatParameters, floatValues, writeAPI)

				writeAPI.Flush()

			//HOME_POSITION
			case 242:
				parameters, msgName := getParameterNames(msgID, mavlinkCommon)

				//one array
				floatValues := convertToFloats(rawValues[0:6], msgID)
				floatValues = append(floatValues, convertToFloats(rawValues[10:], msgID)...)	
				floatParameters := parameters[0:6]
				floatParameters = append(floatParameters, parameters[7:]...)
				writeToInflux(msgID, msgName, floatParameters, floatValues, writeAPI)

			// STATUSTEXT
			case 253:
				parameters, msgName := getParameterNames(msgID, mavlinkCommon)

				floatValues := convertToFloats(rawValues[len(rawValues)-2:], msgID)
				floatParameters := parameters[len(parameters)-2:]
				writeToInflux(msgID, msgName, floatParameters, floatValues, writeAPI)



	
			//ardupilot dialectmessages found in ardupilotmega.xml
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

