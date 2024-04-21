package telemetry

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os/exec"

	"internal/typedef"
	"internal/utils"
)

type SystemBoardInfo struct {
	// Manufacturer string
	// Product      string
	// Hostname     string
	Serial string
	// Release      struct {
	// 	Revision string
	// 	Version  string
	// }
}

func GetSystemBoardInfo() (info typedef.SystemBoard, err error) {
	cmd := exec.Command("ubus", "call", "system", "board")
	byteArr, err := cmd.Output()
	if err != nil {
		err = fmt.Errorf("%s %s", "Command", err.Error())
		return
	}

	var jsonInfo SystemBoardInfo
	err = json.Unmarshal([]byte(byteArr), &jsonInfo)
	if err != nil {
		err = fmt.Errorf("%s %s", "Unmarshal", err.Error())
		return
	}

	err = utils.ConvertFieldsStructToByte(jsonInfo, &info)
	if err != nil {
		err = fmt.Errorf("%s %s", "ConvertFieldsStructToByte", err.Error())
		return
	}

	return
}

func GetDeviceUptime() (uptime uint64, err error) {
	cmd := exec.Command("cat", "/proc/uptime")
	byteArr, err := cmd.Output()
	if err != nil {
		err = fmt.Errorf("%s %s", "Command", err.Error())
		return
	}

	uptime = binary.LittleEndian.Uint64(byteArr)
	return
}
