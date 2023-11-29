package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

const (
	NETBOX_URL = "http://10.39.70.169:8000/api/"
	TOKEN      = "53bccdc4d527945c9b24b0a5cc5a558e212b3def"
	HEADERS    = "application/json"
)

var httpClient = &http.Client{}

func createRequest(method, url string, body []byte) (*http.Request, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+TOKEN)
	req.Header.Set("Content-Type", HEADERS)
	return req, nil
}

func performRequest(req *http.Request) (*http.Response, error) {
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func updateDevicesData(jsonFile string) {
	// Read data from JSON file
	jsonData, err := ioutil.ReadFile(jsonFile)
	if err != nil {
		log.Fatal(err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		log.Fatal(err)
	}

	deviceNames := []string{}

	for _, value := range data {
		if v, ok := value.(map[string]interface{}); ok {
			for _, v := range v {
				if vv, ok := v.(map[string]interface{}); ok {
					if name, ok := vv["name"].(string); ok {
						deviceNames = append(deviceNames, name)
					}
				}
			}
		}
	}

	client := httpClient

	for _, deviceName := range deviceNames {
		url := NETBOX_URL + "dcim/devices/?name=" + deviceName
		req, err := createRequest("GET", url, nil)
		if err != nil {
			log.Fatal(err)
		}

		response, err := performRequest(req)
		if err != nil {
			log.Fatal(err)
		}
		defer response.Body.Close()

		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Fatal(err)
		}

		var deviceDict map[string]interface{}
		if err := json.Unmarshal(body, &deviceDict); err != nil {
			log.Fatal(err)
		}

		results := deviceDict["results"].([]interface{})
		if len(results) > 0 {
			deviceDict := results[0].(map[string]interface{})
			if strings.ToLower(deviceDict["name"].(string)) == strings.ToLower(deviceName) {
				deviceURL := deviceDict["url"].(string)
				updateData := map[string]interface{}{
					"name":          deviceDict["name"],
					"device_type":   deviceDict["device_type"].(map[string]interface{})["id"],
					"custom_fields": map[string]interface{}{"State": "Reserved"},
				}

				updateDataJSON, err := json.Marshal(updateData)
				if err != nil {
					log.Fatal(err)
				}

				req, err := http.NewRequest("PATCH", deviceURL, bytes.NewBuffer(updateDataJSON))
				if err != nil {
					log.Fatal(err)
				}
				req.Header.Set("Authorization", "Token "+TOKEN)
				req.Header.Set("Content-Type", HEADERS)

				response, err := client.Do(req)
				if err != nil {
					log.Fatal(err)
				}
				defer response.Body.Close()

				if response.StatusCode == http.StatusOK {
					log.Println("Device details updated successfully!")
				} else {
					log.Printf("Error updating device details. Status code: %d\n", response.StatusCode)
				}
			} else {
				log.Printf("Failed to find the device: %s\n", deviceName)
			}
		}
	}

	// Check if the file exists before attempting to delete
	if _, err := os.Stat(jsonFile); err == nil {
		// Delete the file
		err := os.Remove(jsonFile)
		if err != nil {
			log.Fatalf("Error deleting file: %v\n", err)
		}
		log.Printf("File '%s' deleted successfully.\n", jsonFile)
	} else {
		log.Printf("File '%s' does not exist.\n", jsonFile)
	}
}

func getDeviceDetails(deviceName string) map[string]interface{} {
	url := fmt.Sprintf("%sdcim/devices/?name=%s", NETBOX_URL, deviceName)
	req, err := createRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}

	response, err := performRequest(req)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		var result map[string]interface{}
		json.NewDecoder(response.Body).Decode(&result)
		deviceDetails := result["results"].([]interface{})[0].(map[string]interface{})
		return deviceDetails
	} else {
		log.Printf("Error: %d", response.StatusCode)
		return nil
	}
}

func getDevicesDetails() []string {
	url := fmt.Sprintf("%sdcim/devices", NETBOX_URL)
	req, err := createRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}

	response, err := performRequest(req)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		var result map[string]interface{}
		json.NewDecoder(response.Body).Decode(&result)
		deviceDetails := result["results"].([]interface{})
		var deviceList []string
		for _, device := range deviceDetails {
			deviceList = append(deviceList, device.(map[string]interface{})["name"].(string))
		}
		return deviceList
	} else {
		log.Printf("Error: %d", response.StatusCode)
		return nil
	}
}

func getInterfacesDetails() []map[string]interface{} {
	url := fmt.Sprintf("%sdcim/interfaces", NETBOX_URL)
	req, err := createRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}

	response, err := performRequest(req)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		var result map[string]interface{}
		json.NewDecoder(response.Body).Decode(&result)
		interfaceDetails := result["results"].([]interface{})
		var interfaceList []map[string]interface{}
		for _, iface := range interfaceDetails {
			interfaceList = append(interfaceList, iface.(map[string]interface{}))
		}
		return interfaceList
	} else {
		log.Printf("Error: %d", response.StatusCode)
		return nil
	}
}

func getDevicesData() []byte {
	deviceList := getDevicesDetails()
	var listOfDeviceDicts []map[string]interface{}

	for _, deviceName := range deviceList {
		interfaceDict := make([]map[string]interface{}, 0)
		deviceDetails := getDeviceDetails(deviceName)

		if deviceDetails["interface_count"].(float64) > 0 {
			interfaceDetails := getInterfacesDetails()
			for _, iface := range interfaceDetails {
				if iface["device"].(map[string]interface{})["name"].(string) == deviceName {
					if iface["speed"] != nil {
						speed := iface["speed"].(float64)
						switch speed {
						case 100000000:
							iface["speed"] = "S_100G"
						case 200000000:
							iface["speed"] = "S_200G"
						case 400000000:
							iface["speed"] = "S_400G"
						}
					}
					interfaceDict = append(interfaceDict, map[string]interface{}{
						"name": iface["name"],
						"attributes": map[string]interface{}{
							"speed": iface["speed"],
						},
					})
				}
			}
		}

		if deviceDetails != nil {
			if _, ok := deviceDetails["custom_fields"].(map[string]interface{})["State"].(string); ok {
				// Do nothing, state exists
			} else {
				deviceDetails["custom_fields"].(map[string]interface{})["State"] = "None"
			}

			if len(interfaceDict) == 0 {
				interfaceDict = append(interfaceDict, map[string]interface{}{})
			}

			deviceData := map[string]interface{}{
				"Id":           deviceDetails["id"],
				"Name":         deviceDetails["name"],
				"DeviceType":   deviceDetails["device_type"].(map[string]interface{})["model"],
				"Manufacturer": deviceDetails["device_type"].(map[string]interface{})["manufacturer"].(map[string]interface{})["name"],
				"State":        deviceDetails["custom_fields"].(map[string]interface{})["State"],
				"interfaces":   interfaceDict,
			}
			listOfDeviceDicts = append(listOfDeviceDicts, deviceData)
		} else {
			log.Printf("Device '%s' not found.", deviceName)
		}
	}

	result, err := json.MarshalIndent(listOfDeviceDicts, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	return result
}

func getDevicesLinks() []byte {
	interfaceDetails := getInterfacesDetails()

	links := make([]map[string]string, 0)

	for _, iface := range interfaceDetails {
		srcDeviceName := iface["device"].(map[string]interface{})["name"].(string)
		src := srcDeviceName + ":" + iface["name"].(string)

		var dst string

		if linkPeers, ok := iface["link_peers"].([]interface{}); ok && len(linkPeers) > 0 {
			peer := linkPeers[0].(map[string]interface{})
			dstDeviceName := peer["device"].(map[string]interface{})["name"].(string)
			dst = dstDeviceName + ":" + peer["name"].(string)
		}

		if src != "" && dst != "" {
			links = append(links, map[string]string{"src": src, "dst": dst})
		}
	}

	// Deduplicate links
	uniqueLinks := make([]map[string]string, 0)
	seenLinks := make(map[string]struct{})

	for _, link := range links {
		key := link["src"] + link["dst"]
		if _, seen := seenLinks[key]; !seen {
			uniqueLinks = append(uniqueLinks, link)
			seenLinks[key] = struct{}{}
		}
	}

	jsonStr, err := json.MarshalIndent(uniqueLinks, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	return jsonStr
}
