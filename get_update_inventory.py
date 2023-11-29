import requests, json, sys, logging, os

NETBOX_URL = "http://10.39.70.169:8000/api/"
HEADERS = {
    "Authorization": "Token 53bccdc4d527945c9b24b0a5cc5a558e212b3def",
    "Content-Type": "application/json",
}

def get_device_details(device_name):
    # Get device details by name
    url = f"{NETBOX_URL}dcim/devices/?name={device_name}"
    response = requests.get(url, headers=HEADERS)

    if response.status_code == 200:
        device_details = response.json()["results"][0]
        return device_details
    else:
        print(f"Error: {response.status_code}")
        return None
    
def get_devices_details():
    # Get device details by name
    url = f"{NETBOX_URL}dcim/devices"
    response = requests.get(url, headers=HEADERS)

    if response.status_code == 200:        
        device_details = response.json()["results"]
        device_lists = [device['name'] for device in device_details]
        return device_lists
    else:
        logging.error(f"Error: {response.status_code}")
        return None

def get_interfaces_details():
    url = f"{NETBOX_URL}dcim/interfaces"
    response = requests.get(url, headers=HEADERS)

    if response.status_code == 200:        
        interface_details = response.json()["results"]
        # interface_lists = [device['name'] for device in interface_details]
        return interface_details
    else:
        logging.error(f"Error: {response.status_code}")
        return None

def get_devices_data():
    device_lists = get_devices_details()
    # device_name = input("Enter the device name: ")
    list_of_device_dicts = []
    for device_name in device_lists:
        interface_dict = []
        device_details = get_device_details(device_name)
        if device_details['interface_count'] > 0:
            interface_details = get_interfaces_details()
            for interface in interface_details:
                if interface['device']['name'] == device_name:
                    if interface['speed'] == 100000000:
                        interface['speed'] = "S_100G"
                    elif interface['speed'] == 200000000:
                        interface['speed'] = "S_200G"
                    elif interface['speed'] == 400000000:
                        interface['speed'] = "S_400G"
                    interface_dict.append({"name": interface['name'], "attributes": {"speed": interface['speed']},})                   
        if device_details:
            if (device_details['custom_fields']['State']):
                pass
            else:
                device_details['custom_fields']['State'] = "None"
            if interface_dict == []:
                interface_dict = [{}]
            device_data = {'Id': device_details['id'], 'Name': device_details['name'], "DeviceType": device_details['device_type']['model'], 'Manufacturer': device_details['device_type']['manufacturer']['name'], 'State': device_details['custom_fields']['State'], "interfaces": interface_dict}
            list_of_device_dicts.append(device_data)
        else:
            logging.info(f"Device '{device_name}' not found.")
    print(json.dumps(list_of_device_dicts, indent=2))

def update_devices_data(json_file):
    # Read data from JSON file
    with open(json_file) as json_file:
        data = json.load(json_file)
    device_names = []
    # update_data = []
    for key, value in data.items():
        if isinstance(value, dict):
            for k, v in value.items():
                device_names.append(v['name'])                 
    
    for device_name in device_names:    
        url = f"{NETBOX_URL}dcim/devices/?name={device_name}" 
        response = requests.request("GET", url, headers=HEADERS)        
        device_dict = response.json()['results'][0]        
        if device_dict['name'].lower() == device_name.lower():
            device_url = device_dict['url']
            update_data = {'name': device_dict['name'], 'device_type': device_dict['device_type']['id'], 'custom_fields': {"State": "Reserved"}}
            data = json.dumps(update_data)  
            response = requests.request("PATCH", device_url, data=data, headers=HEADERS)
            # response = requests.patch(device_url, json=update_data, headers=HEADERS)
            if response.status_code == 200:
                logging.info('Device details updated successfully!')
            else:
                logging.info(f'Error updating device details. Status code: {response.status_code}')  
        else:
            logging.error("Failed to find the device: {}".format(device_name))   
    # Check if the file exists before attempting to delete
    if os.path.exists(json_file.name):
        # Delete the file
        os.remove(json_file.name)
        logging.info(f"File '{json_file}' deleted successfully.")
    else:
        logging.error(f"File '{json_file}' does not exist.") 

def get_devices_links():
    interface_details = get_interfaces_details()
    links = []
    for interface in interface_details:
        srcdeviceName = interface["device"]["name"]
        src = srcdeviceName + ":" + interface["name"]
        if interface["link_peers"] != []:
            for peer in interface["link_peers"]:
                dstdeviceName = peer["device"]["name"]
                dst =  dstdeviceName + ":" + peer["name"]
                break
        else:
            dst = ""
        if src and dst:
            links.append({"src": src, "dst": dst})
        else:
            links.append({})
    links_list = [v for k, v in enumerate(links) if v not in links[:k]]
    print(json.dumps(links_list, indent=2))
   

if __name__ == "__main__":
    # Check the argument to determine which method to call    
    if len(sys.argv) < 2:
        logging.error("Usage: python get_update_inventory.py <command> [json_file_path]")
        sys.exit(1)

    method_to_execute = sys.argv[1]

    if method_to_execute == "update_devices_data":
        # Check if a JSON file path is provided
        if len(sys.argv) == 3:
            json_file = sys.argv[2]
            update_devices_data(json_file)
        else:
            logging.error("Error: JSON file path is missing.")
    else:
        if method_to_execute == "get_devices_data":
            get_devices_data()
        elif method_to_execute == "get_devices_links":
            get_devices_links()
        else:
            logging.error("Python method not found to execute")

    # update_devices_data("output.json")
    # print(get_devices_links())