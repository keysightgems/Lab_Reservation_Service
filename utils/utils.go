package utils

// go clean -modcache
import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

type Interface struct {
	Name   string `json:"name"`
	Speed  int    `json:"speed"`
	Status string `json:"status"`
	// Add more fields as needed
}
type Devicecount struct {
	Devices map[string]Device `json:"devices"`
}

// Device represents the structure of a device
type Device struct {
	ID         float64       `json:"id"`
	Name       string        `json:"name"`
	State      string        `json:"state"`
	Attributes Attributes    `json:"attributes"`
	Interfaces []interface{} `json:"interfaces"`
	// Add more fields as needed
}

// Inventory represents the inventory details of a device
type Attributes struct {
	// Add more fields as needed
	Vendor string `json:"vendor"`
	Type   string `json:"type"`
}

// Dut represents the structure of the device under "duts" key
type Dut struct {
	Name    string                   `json:"desc"`
	Devices map[string]Device        `json:"devices"`
	Links   []map[string]interface{} `json:"links"`
}

// Counter represents a simple counter for generating auto-increment IDs
type Counter struct {
	Value int
}

func (c *Counter) nextID() int {
	c.Value++
	return c.Value
}

// AddDevice adds a new device with interfaces and an auto-incrementing ID to the provided map
func AddDevice(counter *Counter, devices map[int]Device, id float64, name, deviceType, state string, manufacturer string, interfaces []interface{}) map[int]Device {
	deviceID := counter.nextID()
	devices[deviceID] = Device{
		ID:    id,
		Name:  name,
		State: state,
		Attributes: Attributes{
			// Add inventory details here
			Vendor: strings.ToUpper(manufacturer),
			Type:   deviceType,
		},
		Interfaces: interfaces,
	}
	return devices
}

func createInventory(listOfDicts []map[string]interface{}, linksOfDicts []map[string]interface{}, inventoryFile string, inventoryType string) {
	// Parse JSON output into Go data structure

	// Initialize an empty map for devices
	devices := make(map[int]Device)
	// var devicesSlice map[string]Device
	devicesSlice := make(map[string]Device)
	// var atesSlice []Device
	for _, dict := range listOfDicts {
		// Get values using keys
		id := dict["Id"].(float64)
		name := dict["Name"].(string)
		deviceType := dict["DeviceType"].(string)
		manufacturer := dict["Manufacturer"].(string)
		state := dict["State"].(string)
		interfaces := dict["interfaces"].([]interface{})

		idCounter := &Counter{}
		if strings.ToLower(inventoryType) == "all" {
			devices = AddDevice(idCounter, devices, id, name, deviceType, state, manufacturer, interfaces)
		} else {
			if strings.ToLower(state) != "reserved" {
				devices = AddDevice(idCounter, devices, id, name, deviceType, state, manufacturer, interfaces)
			} else {
				devices = make(map[int]Device)
			}
		}

		// Convert the map to a slice
		if deviceType == "DUT" || deviceType == "ATE" || deviceType == "TGEN" {
			for _, device := range devices {
				// devicesSlice = append(devicesSlice, device)
				devicesSlice[device.Name] = device
			}
		}
	}

	// Create Dut with devices
	duts := Dut{
		Name:    "Inventory",
		Devices: devicesSlice,
		Links:   linksOfDicts,
	}

	// Marshal the Dut into JSON
	dutsJSON, err := json.MarshalIndent(duts, "", "    ")
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}

	// Write JSON to a file
	err = ioutil.WriteFile(inventoryFile, dutsJSON, 0644)
	if err != nil {
		fmt.Println("Error writing JSON to file:", err)
		return
	}

	fmt.Println("JSON written to ", inventoryFile)
}

// FileExists checks if a file exists
func FileExists(filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false, fmt.Errorf("file does not exist: %s", filePath)
	}
	return true, nil
}

func UpdateInventory() {
	filePath := "output.json"
	exists, err := FileExists(filePath)
	if err != nil {
		log.Fatal(err) // Log the error and terminate the program
	}
	if exists {
		updateDevicesData(filePath)
		if err != nil {
			fmt.Println("Error executing Python script:", err)
			return
		} else {
			fmt.Println("Device details updated successfully on Netbox as per testbed details.")
		}
	} else {
		log.Fatal("Output file does not exist.")
	}
}
func GetCreateInvFromNetbox() {
	output := getDevicesData()
	var listOfDicts []map[string]interface{}
	err := json.Unmarshal(output, &listOfDicts)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}
	linksoutput := getDevicesLinks()
	var linksOfDicts []map[string]interface{}
	err = json.Unmarshal(linksoutput, &linksOfDicts)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}
	createInventory(listOfDicts, linksOfDicts, "inventory_global.json", "all")
	createInventory(listOfDicts, linksOfDicts, "inventory.json", "NA")
}
