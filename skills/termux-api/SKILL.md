---
name: termux-api
description: "Control Android device features using Termux API - battery, camera, clipboard, location, notifications, SMS, sensors, and more."
metadata: {"pepebot":{"emoji":"📱","requires":{"bins":["termux-battery-status"]},"install":[{"id":"termux","kind":"pkg","package":"termux-api","bins":["termux-battery-status","termux-notification","termux-clipboard-get"],"label":"Install Termux API (pkg install termux-api)"}],"platform":"android"}}
---

# Termux API Skill 📱

Control your Android device through Termux API commands. This skill allows you to interact with Android hardware and system features programmatically.

## Prerequisites

**Required:**
1. Termux API app installed from F-Droid (not Google Play)
2. Termux API package: `pkg install termux-api`
3. Grant necessary Android permissions when prompted

**Installation:**
```bash
pkg install termux-api
```

**Note:** Some commands require Android permissions. Grant them when prompted or via Settings → Apps → Termux:API → Permissions.

---

## 🔋 Battery & Power

### Get Battery Status
```bash
termux-battery-status
```
Returns JSON with battery percentage, temperature, status (charging/discharging), plugged state, etc.

**Example:**
```bash
# Check if battery is low
termux-battery-status | jq '.percentage'
```

---

## 📸 Camera

### Take Photo
```bash
termux-camera-photo -c <camera-id> <output-file>
```

**Parameters:**
- `-c <id>`: Camera ID (0=back, 1=front)
- Output file path

**Examples:**
```bash
# Take photo with back camera
termux-camera-photo -c 0 ~/photo.jpg

# Take selfie with front camera
termux-camera-photo -c 1 ~/selfie.jpg
```

### List Available Cameras
```bash
termux-camera-info
```

---

## 📋 Clipboard

### Get Clipboard Content
```bash
termux-clipboard-get
```

### Set Clipboard Content
```bash
termux-clipboard-set "text to copy"
```

**Examples:**
```bash
# Copy file contents to clipboard
cat notes.txt | termux-clipboard-set

# Paste clipboard to file
termux-clipboard-get > clipboard.txt
```

---

## 📞 Contacts

### List All Contacts
```bash
termux-contact-list
```
Returns JSON array of contacts with names, numbers, emails.

**Example:**
```bash
# Find contact by name
termux-contact-list | jq '.[] | select(.name | contains("John"))'

# Count total contacts
termux-contact-list | jq '. | length'
```

---

## 💬 Dialog & User Input

### Show Dialog
```bash
# Text input dialog
termux-dialog text -t "Enter your name"

# Confirm dialog
termux-dialog confirm -t "Are you sure?"

# Date picker
termux-dialog date -t "Select date"

# Time picker
termux-dialog time -t "Select time"

# Radio buttons
termux-dialog radio -t "Choose one" -v "Option 1,Option 2,Option 3"

# Checkbox
termux-dialog checkbox -t "Select items" -v "Item 1,Item 2,Item 3"

# Spinner/dropdown
termux-dialog spinner -t "Select" -v "Choice 1,Choice 2,Choice 3"
```

**Example:**
```bash
# Get user input
name=$(termux-dialog text -t "Enter name" | jq -r '.text')
echo "Hello, $name!"
```

---

## 📍 Location (GPS)

### Get Current Location
```bash
termux-location -p <provider> -r <request>
```

**Providers:**
- `gps`: GPS hardware (more accurate, slower, outdoor)
- `network`: Network/WiFi (faster, less accurate)
- `passive`: Use cached location

**Request types:**
- `once`: Get location once (default)
- `last`: Get last known location
- `updates`: Continuous updates

**Examples:**
```bash
# Get GPS location
termux-location -p gps -r once

# Quick network location
termux-location -p network -r last

# Parse coordinates
termux-location -p network -r once | jq -r '.latitude, .longitude'
```

---

## 🔊 Media & Audio

### Media Player
```bash
# Play audio file
termux-media-player play <file>

# Pause playback
termux-media-player pause

# Info about current track
termux-media-player info
```

### Text-to-Speech
```bash
termux-tts-speak "Hello from Pepebot"

# Different language
termux-tts-speak -l en-US "Hello"

# Different speech rate and pitch
termux-tts-speak -r 0.5 -p 2.0 "Slow and high pitch"
```

**Parameters:**
- `-l`: Language (en-US, id-ID, etc.)
- `-r`: Speech rate (0.1-2.0)
- `-p`: Pitch (0.1-2.0)
- `-e`: Engine name
- `-s`: Stream type

### Speech-to-Text
```bash
termux-speech-to-text
```
Records audio and converts to text (requires microphone permission).

---

## 🔔 Notifications

### Display Notification
```bash
termux-notification -t "Title" -c "Content"
```

**Parameters:**
- `-t`: Title
- `-c`: Content/message
- `-i`: ID (for updating/removing)
- `--priority`: min/low/default/high/max
- `--sound`: Play notification sound
- `--vibrate`: Vibrate pattern (e.g., "200,100,200")
- `--action`: Add action button
- `--on-delete`: Command to run on dismiss

**Examples:**
```bash
# Simple notification
termux-notification -t "Pepebot" -c "Task completed!"

# Notification with sound and vibration
termux-notification -t "Alert" -c "Important!" --sound --vibrate "500,200,500"

# Persistent notification
termux-notification -i 123 -t "Bot Running" -c "Click to stop" --ongoing

# Remove notification
termux-notification-remove 123
```

---

## 💾 Storage

### Get Storage Info
```bash
termux-storage-get
```
Request access to external storage (shows permission dialog).

**After granting permission, access shared storage at:**
- `~/storage/shared/` - Internal storage
- `~/storage/external-1/` - SD card (if available)
- `~/storage/downloads/` - Downloads folder
- `~/storage/dcim/` - Camera photos
- `~/storage/pictures/` - Pictures folder

---

## 📱 Device Info

### WiFi Connection Info
```bash
termux-wifi-connectioninfo
```
Returns SSID, IP address, link speed, frequency, etc.

**Example:**
```bash
# Get current WiFi name
termux-wifi-connectioninfo | jq -r '.ssid'

# Check signal strength
termux-wifi-connectioninfo | jq '.rssi'
```

### WiFi Scan
```bash
termux-wifi-scaninfo
```
Scan for available WiFi networks.

### Sensor Info
```bash
termux-sensor -l
```
List available sensors (accelerometer, gyroscope, light, proximity, etc.)

### Read Sensor Data
```bash
termux-sensor -s <sensor-name> -n <count>
```

**Example:**
```bash
# Read accelerometer
termux-sensor -s "BMI160 accelerometer" -n 1

# Monitor light sensor
termux-sensor -s "Light Sensor" -n 10
```

---

## 📞 Telephony

### Send SMS
```bash
termux-sms-send -n <number> "message"
```

**Example:**
```bash
termux-sms-send -n "+1234567890" "Hello from Pepebot!"
```

**⚠️ Note:** Requires SMS permission. May not work on all devices/carriers.

### Get SMS List
```bash
termux-sms-list -l <limit>
```

**Parameters:**
- `-l`: Limit number of messages
- `-n`: Phone number filter
- `-t`: Type (all/inbox/sent/draft)
- `-d`: Show date

### Make Phone Call
```bash
termux-telephony-call <number>
```

### Get Device Info
```bash
termux-telephony-deviceinfo
```
Returns device ID, software version, phone type, etc.

---

## 📲 System

### Brightness Control
```bash
# Set brightness (0-255)
termux-brightness 128

# Get current brightness
termux-brightness
```

### Volume Control
```bash
# Set volume
termux-volume <stream> <level>

# Streams: alarm, music, notification, ring, system, call
termux-volume music 10
```

### Vibrate
```bash
# Vibrate for duration (milliseconds)
termux-vibrate -d 1000

# Vibrate pattern
termux-vibrate -f 500,200,500,200
```

### Toast Message
```bash
termux-toast "Hello from Pepebot"

# Short or long duration
termux-toast -s "Short toast"
termux-toast -l "Long toast"

# Different position
termux-toast -g top "Top toast"
termux-toast -g middle "Middle toast"
termux-toast -g bottom "Bottom toast"
```

### Share Content
```bash
# Share file
termux-share <file>

# Share text
echo "Hello" | termux-share

# Choose specific app
termux-share -a org.telegram.messenger <file>
```

### Open URL
```bash
termux-open-url <url>
```

**Example:**
```bash
termux-open-url "https://google.com"
termux-open-url "tel:+1234567890"
termux-open-url "mailto:user@example.com"
```

---

## 🎯 Common Use Cases

### 1. Battery Monitor
```bash
# Alert when battery is low
battery=$(termux-battery-status | jq '.percentage')
if [ $battery -lt 20 ]; then
    termux-notification -t "Low Battery" -c "Only $battery% remaining"
fi
```

### 2. Location Logger
```bash
# Log current location
location=$(termux-location -p network -r once)
echo "$(date): $location" >> location_log.txt
```

### 3. Smart Notifications
```bash
# Notify when someone mentions you
termux-notification \
    -t "New Message" \
    -c "John mentioned you" \
    --sound \
    --vibrate "200,100,200" \
    --action "termux-open-url 'telegram://'"
```

### 4. Voice Assistant
```bash
# Voice command input
text=$(termux-speech-to-text)
termux-tts-speak "You said: $text"
```

### 5. Auto Photo
```bash
# Take photo every hour
while true; do
    termux-camera-photo -c 0 ~/photos/$(date +%Y%m%d_%H%M%S).jpg
    sleep 3600
done
```

### 6. Clipboard Sync
```bash
# Sync clipboard to file
termux-clipboard-get > ~/.clipboard_backup
```

### 7. WiFi Monitor
```bash
# Check WiFi connection
ssid=$(termux-wifi-connectioninfo | jq -r '.ssid')
if [ "$ssid" = "MyHomeWiFi" ]; then
    echo "At home"
else
    echo "Away from home"
fi
```

---

## 🔐 Permissions

Required Android permissions (grant when prompted):

- **Camera**: `termux-camera-*`
- **Location**: `termux-location`
- **Microphone**: `termux-speech-to-text`
- **Contacts**: `termux-contact-list`
- **SMS**: `termux-sms-*`
- **Phone**: `termux-telephony-*`
- **Storage**: `termux-storage-get`
- **Sensors**: `termux-sensor`

**Grant permissions:**
Settings → Apps → Termux:API → Permissions → Enable required permissions

---

## 💡 Tips

1. **Check Command Availability**: Run `which <command>` to verify installation
2. **JSON Parsing**: Use `jq` for parsing JSON output
3. **Error Handling**: Check exit codes with `$?`
4. **Permissions**: Grant all permissions at once in Android settings
5. **Background Tasks**: Use `tmux` to keep scripts running
6. **Logging**: Pipe output to files for debugging

---

## 🐛 Troubleshooting

**Command not found:**
```bash
pkg install termux-api
```

**Permission denied:**
- Go to Android Settings → Apps → Termux:API
- Enable required permissions
- Try command again

**Camera not working:**
- Close all apps using camera
- Grant camera permission
- Try different camera ID

**Location not available:**
- Enable Location in Android settings
- Grant location permission to Termux:API
- Try different provider (gps/network)

---

## 🔗 References

- [Termux Wiki](https://wiki.termux.com/wiki/Termux:API)
- [Termux API GitHub](https://github.com/termux/termux-api)
- [Android Permissions](https://developer.android.com/guide/topics/permissions/overview)

---

**Note:** This skill is only available when running Pepebot on Android via Termux. Commands will fail on other platforms.
