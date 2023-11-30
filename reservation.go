package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"lablrs/utils"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	graph "github.com/openconfig/ondatra/binding/portgraph"
)

var inventoryConfig Inventory
var inventory graph.ConcreteGraph
var configNodesToDevices map[*graph.ConcreteNode]Device
var configPortsToPorts map[*graph.ConcretePort]Interface

type Inventory struct {
	Desc    string            `json:"desc"`
	Devices map[string]Device `json:"devices"`
	Links   []Link            `json:"links"`
}

type Testbed struct {
	Desc    string             `json:"desc"`
	Devices map[string]BDevice `json:"devices"`
	Links   []Link             `json:"links"`
}

type Device struct {
	Name       string            `json:"name"`
	Attrs      map[string]string `json:"attributes"`
	Services   []Service         `json:"services"`
	Interfaces []Interface       `json:"interfaces"`
}

type Service struct {
	Name          string `json:"name"`
	AddressFamily string `json:"address_family"`
	Address       string `json:"address"`
	Protocol      string `json:"protocol"`
	Port          int    `json:"port"`
}

type Interface struct {
	Name  string            `json:"name"`
	Attrs map[string]string `json:"attributes"`
}

type Link struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}

type BDevice struct {
	Name  string            `json:"name"`
	Attrs map[string]string `json:"attributes"`
	Ports map[string]Port   `json:"ports"`
}

type Port struct {
	Name  string            `json:"name"`
	Attrs map[string]string `json:"attributes"`
}

type InputAttributes struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type InputInterface struct {
	Attributes []InputAttributes `json:"attributes,omitempty"`
	Name       string            `json:"name"`
	Speed      string            `json:"speed,omitempty"`
}

type InputDevice struct {
	Interfaces []InputInterface `json:"interfaces"`
	Model      string           `json:"model"`
	Name       string           `json:"name"`
	Vendor     string           `json:"vendor"`
}

type InputLink struct {
	Dst string `json:"dst"`
	Src string `json:"src"`
}

type InputData struct {
	Devices []InputDevice `json:"devices"`
	Links   []InputLink   `json:"links"`
}

func uploadInventory() {
	loadConcreteGraph()
}

func ConvertData(srcData InputData) Testbed {
	destData := Testbed{
		Desc:    "testbed",
		Devices: make(map[string]BDevice),
	}
	deviceNameMap := make(map[string]string)
	for _, srcDevice := range srcData.Devices {
		destDevice := BDevice{
			Name:  srcDevice.Name,
			Attrs: make(map[string]string),
			Ports: make(map[string]Port),
		}

		// Process device attributes
		if srcDevice.Vendor != "" {
			destDevice.Attrs["vendor"] = srcDevice.Vendor
		}

		// Process interfaces
		for _, srcInterface := range srcDevice.Interfaces {
			destPort := Port{
				Name:  srcInterface.Name,
				Attrs: make(map[string]string),
			}

			// Process interface attributes
			for _, srcAttr := range srcInterface.Attributes {
				destPort.Attrs[srcAttr.Name] = srcAttr.Value
			}

			// Process speed attribute
			if srcInterface.Speed != "" {
				destPort.Attrs["speed"] = srcInterface.Speed
			}

			destDevice.Ports[srcInterface.Name] = destPort
		}

		destData.Devices[srcDevice.Name] = destDevice
		// Create a mapping of old device names to new ones
		deviceNameMap[srcDevice.Name] = destDevice.Name
	}

	// Process links
	for _, srcLink := range srcData.Links {
		// Update device names in the links based on the mapping
		srcDeviceName := parseLink(srcLink.Src)
		dstDeviceName := parseLink(srcLink.Dst)

		srcDeviceName = deviceNameMap[srcDeviceName]
		dstDeviceName = deviceNameMap[dstDeviceName]
		srcPortName := srcLink.Src
		dstPortName := srcLink.Dst

		destLink := Link{
			Src: fmt.Sprintf("%s:%s", srcDeviceName, srcPortName),
			Dst: fmt.Sprintf("%s:%s", dstDeviceName, dstPortName),
		}
		destData.Links = append(destData.Links, destLink)
	}

	return destData
}

// parseLink splits the link string into device and port parts
func parseLink(link string) string {
	parts := splitLink(link)
	// return parts[0], parts[1]  // here return both device and interface names
	return parts[0]
}

// splitLink splits the link string into device and port parts
func splitLink(link string) []string {
	return split(link, "_")
}

// split is a helper function to split a string based on a separator
func split(s, sep string) []string {
	return append([]string{}, append([]string(nil), splitSlice(s, sep)...)...)
}

// splitSlice is a helper function to split a string based on a separator
func splitSlice(s, sep string) []string {
	i := 0
	for {
		if i+len(sep) > len(s) {
			break
		}
		if s[i:i+len(sep)] == sep {
			return append([]string{s[:i]}, splitSlice(s[i+len(sep):], sep)...)
		}
		i++
	}
	return append([]string{s}, []string{}...)
}

func loadConcreteGraph() {
	nodes := []*graph.ConcreteNode{}
	edges := []*graph.ConcreteEdge{}
	portPointers := map[string]*graph.ConcretePort{}
	for dname, device := range inventoryConfig.Devices {
		ports := []*graph.ConcretePort{}
		for _, port := range device.Interfaces {
			if port.Attrs == nil {
				port.Attrs = map[string]string{"reserved": "no"}
			}
			port.Attrs["reserved"] = "no"
			newPort := &graph.ConcretePort{Desc: (dname + ":" + port.Name), Attrs: port.Attrs}
			ports = append(ports, newPort)
			configPortsToPorts[newPort] = port
			portPointers[dname+":"+port.Name] = newPort
		}
		if device.Attrs == nil {
			device.Attrs = map[string]string{}
		}
		device.Attrs["reserved"] = "no"
		newNode := &graph.ConcreteNode{Desc: dname, Ports: ports, Attrs: device.Attrs}
		nodes = append(nodes, newNode)
		configNodesToDevices[newNode] = device
	}
	inventory.Nodes = nodes
	for _, link := range inventoryConfig.Links {
		edges = append(edges, &graph.ConcreteEdge{Src: portPointers[link.Src], Dst: portPointers[link.Dst]})
	}
	inventory.Edges = edges
}

func reserve(c *gin.Context) {
	testbedData := InputData{}
	if err := c.BindJSON(&testbedData); err != nil {
		return
	}

	testbedConfig := ConvertData(testbedData)

	testbed := graph.AbstractGraph{}
	loadAbstractGraph(testbedConfig, &testbed)
	assignment, err := graph.Solve(context.Background(), &testbed, &inventory)
	if err != nil {
		return
	}
	devices := map[string]BDevice{}
	for _, node := range testbed.Nodes {
		ports := map[string]Port{}
		for _, port := range node.Ports {
			configPortsToPorts[assignment.Port2Port[port]].Attrs["reserved"] = "yes"
			newPort := Port{Name: assignment.Port2Port[port].Desc, Attrs: assignment.Port2Port[port].Attrs}
			ports[port.Desc] = newPort
		}
		configNodesToDevices[assignment.Node2Node[node]].Attrs["reserved"] = "yes"
		newNode := BDevice{Name: assignment.Node2Node[node].Desc, Attrs: assignment.Node2Node[node].Attrs, Ports: ports}
		devices[node.Desc] = newNode
	}
	links := []Link{}
	for _, edge := range testbed.Edges {
		newLink := Link{Src: assignment.Port2Port[edge.Src].Desc, Dst: assignment.Port2Port[edge.Dst].Desc}
		links = append(links, newLink)
	}
	content, _ := json.Marshal(Testbed{Devices: devices, Links: links})
	err = ioutil.WriteFile("output.json", content, 0644)
	if err != nil {
		log.Fatal(err)
	}
	utils.UpdateInventory()
	c.IndentedJSON(http.StatusCreated, Testbed{Devices: devices, Links: links})
}

func loadAbstractGraph(testbedConfig Testbed, testbed *graph.AbstractGraph) {
	nodes := []*graph.AbstractNode{}
	edges := []*graph.AbstractEdge{}
	portPointers := map[string]*graph.AbstractPort{}
	for dname, device := range testbedConfig.Devices {
		ports := []*graph.AbstractPort{}
		for pid, port := range device.Ports {
			if port.Attrs == nil {
				port.Attrs = map[string]string{"reserved": "no"}
			}
			port.Attrs["reserved"] = "no"
			portConstraints := map[string]graph.PortConstraint{}
			for aid, attribute := range port.Attrs {
				portConstraints[aid] = graph.Equal(attribute)
			}
			newPort := &graph.AbstractPort{Desc: (dname + ":" + pid), Constraints: portConstraints}
			ports = append(ports, newPort)
			portPointers[dname+":"+pid] = newPort
		}
		if device.Attrs == nil {
			device.Attrs = map[string]string{}
		}
		device.Attrs["reserved"] = "no"
		deviceConstraints := map[string]graph.NodeConstraint{}
		for aid, attribute := range device.Attrs {
			deviceConstraints[aid] = graph.Equal(attribute)
		}
		newNode := &graph.AbstractNode{Desc: dname, Ports: ports, Constraints: deviceConstraints}
		nodes = append(nodes, newNode)
	}
	testbed.Nodes = nodes
	for _, link := range testbedConfig.Links {
		edges = append(edges, &graph.AbstractEdge{Src: portPointers[link.Src], Dst: portPointers[link.Dst]})
	}
	testbed.Edges = edges
}

func main() {
	utils.GetCreateInvFromNetbox()
	filePath := "inventory.json"
	jsonData, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}

	// Unmarshalling JSON data into the inventoryConfig object
	err = json.Unmarshal(jsonData, &inventoryConfig)
	if err != nil {
		fmt.Println("Error unmarshalling JSON:", err)
		return
	}
	inventory = graph.ConcreteGraph{}
	configNodesToDevices = map[*graph.ConcreteNode]Device{}
	configPortsToPorts = map[*graph.ConcretePort]Interface{}
	uploadInventory()
	// reserve()
	router := gin.Default()
	router.POST("/reserve", reserve)
	router.Run(":8080")
}
