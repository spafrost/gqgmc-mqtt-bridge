
> **🤖AI AUTHORED SOFTWARE WARNING🤖**
> 
> **A lot of this project, including all source code and documentation, was created using artificial intelligence assistance.**
> 
> **⚠️ CRITICAL SAFETY REMINDER:**
> - **Review all code thoroughly** before deploying in any environment
> - **Test extensively** with your specific hardware configuration  
> - **Verify security measures** are appropriate for your use case
> - **Radiation monitoring is safety-critical** - validate data accuracy independently
> - **No warranties or guarantees given or implied** for any code in this repository.
> 
> **Use at your own risk. The author(s) assume no responsibility for any issues, data loss, security vulnerabilities, or safety concerns that may arise from using this AI-generated software.**

## Overview

HTTP to MQTT bridge that emulates the GQ Electronics GMC series geiger counter reporting endpoint used for their crowd sourced radiation map, enabling private data collection for home automation and monitoring systems.

Instead of sending data to the manufacturer's cloud service, you can configure your geiger counter to report to this local endpoint, which will forward the readings to your chosen MQTT broker.

**Key features:**
- 🏠 Works with home automation systems (Home Assistant, OpenHAB, etc.)
- 📊 Private data collection and monitoring
- 🔧 Enables custom alerting and notification systems
- 🏢 Supports multiple devices 


## Supported Geiger Counters

> ⚠️ **Important:** This service has been tested with limited models. Compatibility with untested models is theoretical based on WiFi reporting capabilities.

| Model | Status | WiFi Support | Notes |
|-------|--------|--------------|-------|
| **GMC-500+** | ✅ **Tested** | Yes | Confirmed working with WiFi data logging |
| **GMC-600+** | 🔶 **Theoretical** | Yes | Professional model - should work but unverified |
| **GMC-800** | 🔶 **Theoretical** | Yes | Latest generation - should work but unverified |
| **GMC-320+** | 🔶 **Theoretical** | Yes | Entry-level WiFi model - should work but unverified |
| **GMC-300** series | ❌ **Incompatible** | No | Lacks WiFi reporting capability |


> **Help Expand Compatibility:** If you successfully test this with other GMC models, please report your results to help improve this compatibility table.

> **Reference:** [GQ Electronics GMC Series](https://www.gqelectronicsllc.com/comersus/store/comersus_listItems.asp?idCategory=2) geiger counters support WiFi data logging to custom servers.

## How It Works

```
┌─────────────────┐    HTTP GET     ┌──────────────┐    MQTT Publish    ┌─────────────┐
│  GMC Geiger     │ ──────────────► │  GQGMC Proxy │ ──────────────────► │ MQTT Broker │
│  Counter        │  /report?...    │  (This App)  │  topic/CPM: "42"   │ (Your Choice)│
└─────────────────┘                 └──────────────┘                     └─────────────┘
```

### Original GMC Reporting Flow:
```
GMC Counter ──► gqgmc.com (Manufacturer's Radiation Map)
```

### With GQGMC Proxy:
```
GMC Counter ──► Your Local Server ──► Your MQTT System ──► Your Applications
```

## Supported Geiger Counter Parameters

The service only accepts the same HTTP parameters that GMC counters send to the official endpoint:

| Parameter | Type | Description | Example |
|-----------|------|-------------|---------|
| `GID` | String | Geiger counter device ID (alphanumeric, max 50 chars) | `GMC1` |
| `CPM` | Integer | Current counts per minute | `42` |
| `ACPM` | Float | Average counts per minute over time | `38.5` |
| `uSV` | Float | Microsieverts per hour (radiation dose) | `0.15` |
| `AID` | String | Additional identifier (alphanumeric, max 50 chars) | `AREA1` |

> **Note:** Parameter validation ensures only properly formatted radiation data is processed, rejecting malformed or potentially malicious requests.

## Quick Start

### 1. Using Docker (Recommended)

```bash
docker run -d -p 80:80 \
  --name gqgmc-proxy \
  --restart=unless-stopped \
  -e MQTT_BROKER="tcp://your-mqtt-broker:1883" \
  -e MQTT_TOPIC="your-chosen-base-topic" \
  -e MQTT_USERNAME="mqtt-username" \
  -e MQTT_PASSWORD="mqtt-password" \
  gqgmcproxy
```

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `MQTT_BROKER` | `tcp://localhost:1883` | MQTT broker connection string |
| `MQTT_TOPIC` | `params` | Base topic for MQTT messages |
| `MQTT_USERNAME` | *(none)* | MQTT authentication username |
| `MQTT_PASSWORD` | *(none)* | MQTT authentication password |

### 2. Configure Your GMC Geiger Counter

Access your GMC device's WiFi settings via the **"Server"** option int he main menu and configure the server endpoint:


- **Website:** `docker-container-ip-or-fqdn` (e.g., `192.168.1.100/docker.local`)
- **URL:** `<blank>` (leave value blank)
- **User ID:** `this-will-be-the-AID-value`
- **Counter ID:** `this-will-be-the-GID-value` 
- **Period:** `60` seconds (recommended)

> **Reference:** See [GMC WiFi Setup Guide](https://www.gqelectronicsllc.com/GMC_Data_logging.htm) for detailed configuration instructions.

### 3. Multiple Device Setup

**For multiple geiger counters:**

1. **Ensure unique device IDs:** Each  device should have a unique `Counter ID` identifier
2. **Use the same server configuration:** All devices can point to the same docker instance
3. **Data separation:** Readings are automatically organized by device ID in MQTT topics

**Example device configuration:**
```
Device 1:  Counter ID = GMC1
Device 2:  Counter ID = GMC2
Device 3:  Counter ID = GMC3
```

**Resulting MQTT topics:**
```
radiation/GMC1/CPM → "42"
radiation/GMC2/CPM → "38" 
radiation/GMC3/CPM → "51"
```

### 4. Verify Data Flow

Check that data is being received:
```bash
# Monitor container logs
docker logs -f <container-name>

# Expected output:
connection from 192.168.1.50:12345 to / with 4 params  
published to topic radiation/GMC1/CPM: 42
published to topic radiation/GMC1/uSV: 0.15
```

### Verify Data on MQTT 

Data is published to device-specific subtopics:


**Example with topic `radiation` and device `GMC1`:**
```
radiation/GMC1/GID  → "GMC1"
radiation/GMC1/CPM  → "42"
radiation/GMC1/ACPM → "38.5"  
radiation/GMC1/uSV  → "0.15"
radiation/GMC1/AID  → "AREA1"
```

**Multiple device example:**
```
radiation/GMC1/CPM → "42"     # Device 1 in living room
radiation/GMC2/CPM → "38"     # Device 2 in basement  
radiation/GMC3/CPM → "51"     # Device 3 in garage
```

> **Device ID Extraction:** The device ID is automatically extracted from the `GID` parameter sent by each geiger counter. If no `GID` is provided, readings are stored under `unknown` device.

## Integration Examples

### Home Assistant

Configure sensors for each geiger counter device:

```yaml
mqtt:
  sensor:
    # Device 1 - Living Room Counter
    - name: "Living Room Geiger CPM"
      state_topic: "radiation/GMC1/CPM"
      unit_of_measurement: "CPM"
      
    - name: "Living Room Radiation Level"  
      state_topic: "radiation/GMC1/uSV"
      unit_of_measurement: "μSv/h"
      
    # Device 2 - Basement Counter  
    - name: "Basement Geiger CPM"
      state_topic: "radiation/GMC2/CPM"
      unit_of_measurement: "CPM"
      
    - name: "Basement Radiation Level"
      state_topic: "radiation/GMC2/uSV"
      unit_of_measurement: "μSv/h"
```

**Topic structure parsed:**
- `radiation/GMC1/CPM` → `deviceId: "GMC1"`, `parameter: "CPM"`
- `radiation/GMC2/uSV` → `deviceId: "GMC2"`, `parameter: "uSV"`

## Security Features

- 🛡️ **Rate limiting** (10 requests/second) to prevent abuse
- 🔒 **Input validation** with strict parameter format checking  
- 🚫 **Method restrictions** (GET/HEAD only)
- 📏 **Request size limits** (128KB maximum)
- 📊 **Parameter count limits** (5 parameters maximum)
- 🏥 **Health checks** for container orchestration
- 🔐 **HTTP security headers** to prevent common attacks

## Useful Info



### Example Request
```http
GET /?AID=USER1&GID=GMC1&CPM=42&ACPM=38.5&uSV=0.15 HTTP/1.1
```

### Successful Response
```http
HTTP/1.1 200 OK
Content-Type: text/plain

OK.ERR0
```
> **Note** This response is what the counter is expecting when data is successfully reported. Only this response will supress error messages on the device screen. 
### Error Responses
- `400 Bad Request` - Invalid parameters or format, particularly if an invalid GET value is received or too many parameters
- `405 Method Not Allowed` - Only GET/HEAD allowed  
- `429 Too Many Requests` - Rate limit exceeded

## Building from Source

### Prerequisites
- Go 1.20 or later
- Docker (for containerization)
- Make (optional, for build automation)

### Local Development
```bash
# Clone repository
git clone <repository-url>
cd gqgmc-mqtt-bridge

# Install dependencies
go mod tidy

# Run locally
go run .

# Test endpoint
curl "http://localhost:80/?AID=USER1&GID=GMC1&CPM=42&ACPM=38.5&uSV=0.15"
```

### Docker Build
```bash
# Build image
docker build -t gqgmc-mqtt-bridge .

# Run container
docker run -p <port-of-choice>:80 gqgmc-mqtt-bridge
```

## Troubleshooting

### Debug Commands
```bash
# View container logs
docker logs -f gqgmc-proxy

# Check container health
docker inspect gqgmc-proxy --format='{{.State.Health.Status}}'

# Test MQTT connection - monitor all devices
mosquitto_sub -h your-broker -t "radiation/+/+"

# Test MQTT connection - monitor specific device
mosquitto_sub -h your-broker -t "radiation/GMC1/+"

# Test MQTT connection - monitor specific parameter across all devices  
mosquitto_sub -h your-broker -t "radiation/+/CPM"
```

## References

- [GQ Electronics GMC Series](https://www.gqelectronicsllc.com/comersus/store/comersus_listItems.asp?idCategory=2) - Official product pages
- [GMC WiFi Data Logging Guide](https://www.gqelectronicsllc.com/GMC_Data_logging.htm) - Configuration instructions
- [GQ User Manual Downloads](https://www.gqelectronicsllc.com/GQ-RFC1201.htm) - Technical documentation

