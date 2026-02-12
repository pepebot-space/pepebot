# Termux API - Advanced Examples 🚀

Real-world examples and automation scripts using Termux API.

## 📱 Smart Home Automation

### Location-Based Actions
```bash
#!/bin/bash
# auto_home.sh - Trigger actions when arriving/leaving home

HOME_SSID="MyHomeWiFi"
current_ssid=$(termux-wifi-connectioninfo | jq -r '.ssid')

if [ "$current_ssid" = "$HOME_SSID" ]; then
    # At home
    termux-notification -t "Welcome Home!" -c "You are now connected to home WiFi"
    termux-volume music 15
    termux-brightness 200
else
    # Away from home
    termux-volume music 5
    termux-brightness 50
fi
```

### Battery Management
```bash
#!/bin/bash
# battery_alert.sh - Smart battery notifications

battery_info=$(termux-battery-status)
percentage=$(echo $battery_info | jq '.percentage')
status=$(echo $battery_info | jq -r '.status')
temp=$(echo $battery_info | jq '.temperature')

# Low battery alert
if [ $percentage -lt 20 ] && [ "$status" != "CHARGING" ]; then
    termux-notification \
        -t "⚠️ Low Battery" \
        -c "Only $percentage% remaining. Please charge!" \
        --priority high \
        --vibrate "500,200,500"
fi

# Full charge alert
if [ $percentage -ge 95 ] && [ "$status" = "CHARGING" ]; then
    termux-notification \
        -t "🔋 Battery Full" \
        -c "Battery is at $percentage%. Unplug charger." \
        --sound
fi

# Overheat warning
if [ $temp -gt 40 ]; then
    termux-notification \
        -t "🔥 Device Hot" \
        -c "Temperature: ${temp}°C. Stop charging!" \
        --priority max \
        --vibrate "1000,500,1000"
fi
```

## 🤖 AI Assistant Integration

### Voice Command Processor
```bash
#!/bin/bash
# voice_assistant.sh - Voice-controlled Pepebot

termux-tts-speak "Hello, I'm listening"
command=$(termux-speech-to-text)

echo "User said: $command"

# Process command with Pepebot
response=$(~/pepebot agent -m "$command")

# Speak response
termux-tts-speak "$response"

# Show notification
termux-notification -t "Pepebot Response" -c "$response"
```

### Photo Analysis
```bash
#!/bin/bash
# photo_ai.sh - Take photo and analyze with AI

# Take photo
photo_path="/sdcard/temp_photo.jpg"
termux-camera-photo -c 0 $photo_path

# Upload and analyze (example with Pepebot)
termux-notification -t "📸 Analyzing photo..." -c "Please wait"

# Send to Pepebot for analysis
result=$(~/pepebot agent -m "Analyze this image: $photo_path")

# Show result
termux-notification -t "Analysis Result" -c "$result"
termux-tts-speak "$result"
```

## 🔐 Security & Monitoring

### Motion Detection Alert
```bash
#!/bin/bash
# motion_detector.sh - Alert on device movement

while true; do
    # Read accelerometer
    accel=$(termux-sensor -s "Accelerometer" -n 1)

    x=$(echo $accel | jq '.[0].values[0]')
    y=$(echo $accel | jq '.[0].values[1]')
    z=$(echo $accel | jq '.[0].values[2]')

    # Check if moved significantly
    total=$(echo "$x * $x + $y * $y + $z * $z" | bc)
    threshold=100

    if [ $(echo "$total > $threshold" | bc) -eq 1 ]; then
        # Device moved!
        termux-camera-photo -c 1 ~/security/$(date +%s).jpg
        termux-notification -t "⚠️ Motion Detected!" -c "Device was moved"

        # Send location
        location=$(termux-location -p network -r once)
        echo "$(date): Motion at $location" >> ~/security/log.txt
    fi

    sleep 5
done
```

### SMS Security Alert
```bash
#!/bin/bash
# sms_alert.sh - Send SMS on security events

ALERT_NUMBER="+1234567890"

# Monitor for suspicious activity
suspicious_process=$(ps aux | grep suspicious_app)

if [ ! -z "$suspicious_process" ]; then
    location=$(termux-location -p network -r once | jq -r '.latitude, .longitude')

    termux-sms-send -n $ALERT_NUMBER \
        "ALERT: Suspicious activity detected on device. Location: $location"
fi
```

## 📊 Data Collection

### Location Logger
```bash
#!/bin/bash
# location_tracker.sh - Log location history

LOG_FILE=~/logs/location_history.json

while true; do
    location=$(termux-location -p network -r once)
    timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    # Add timestamp to location data
    echo "$location" | jq --arg time "$timestamp" '. + {timestamp: $time}' >> $LOG_FILE

    # Upload to server (optional)
    # curl -X POST -d "$location" https://your-server.com/api/location

    sleep 3600  # Log every hour
done
```

### WiFi Network Scanner
```bash
#!/bin/bash
# wifi_scanner.sh - Map WiFi networks

OUTPUT_FILE=~/logs/wifi_networks.json

# Scan WiFi
networks=$(termux-wifi-scaninfo)

# Get current location
location=$(termux-location -p network -r once)

# Combine data
combined=$(jq -n \
    --argjson networks "$networks" \
    --argjson location "$location" \
    '{timestamp: now, location: $location, networks: $networks}')

echo "$combined" >> $OUTPUT_FILE

termux-notification -t "WiFi Scan Complete" -c "Found $(echo $networks | jq '. | length') networks"
```

### Battery Life Analyzer
```bash
#!/bin/bash
# battery_monitor.sh - Track battery usage patterns

LOG_FILE=~/logs/battery_log.csv

# Create header if file doesn't exist
if [ ! -f $LOG_FILE ]; then
    echo "timestamp,percentage,temperature,status,health" > $LOG_FILE
fi

while true; do
    battery=$(termux-battery-status)

    timestamp=$(date +"%Y-%m-%d %H:%M:%S")
    percentage=$(echo $battery | jq '.percentage')
    temp=$(echo $battery | jq '.temperature')
    status=$(echo $battery | jq -r '.status')
    health=$(echo $battery | jq -r '.health')

    echo "$timestamp,$percentage,$temp,$status,$health" >> $LOG_FILE

    sleep 300  # Log every 5 minutes
done
```

## 💬 Communication Automation

### Auto-Reply SMS
```bash
#!/bin/bash
# auto_reply.sh - Automatic SMS responses

AWAY_MESSAGE="I'm currently away. I'll reply when I return."

# Monitor new messages
while true; do
    new_sms=$(termux-sms-list -l 1 -t inbox | jq '.[0]')
    sender=$(echo $new_sms | jq -r '.number')

    # Check if already replied (simple check)
    if ! grep -q "$sender" ~/replied.txt; then
        # Send auto-reply
        termux-sms-send -n "$sender" "$AWAY_MESSAGE"

        # Mark as replied
        echo "$sender" >> ~/replied.txt

        # Notify user
        termux-notification -t "Auto-Reply Sent" -c "Replied to $sender"
    fi

    sleep 60
done
```

### Smart Notification Manager
```bash
#!/bin/bash
# notification_manager.sh - Intelligent notification handling

# Quiet hours: 10 PM - 7 AM
hour=$(date +%H)

if [ $hour -ge 22 ] || [ $hour -lt 7 ]; then
    # Silent mode
    termux-volume notification 0
    termux-notification -t "🌙 Quiet Hours" -c "Notifications muted"
else
    # Normal mode
    termux-volume notification 7
fi

# Filter by location
ssid=$(termux-wifi-connectioninfo | jq -r '.ssid')

if [ "$ssid" = "Office-WiFi" ]; then
    # At office - silent mode
    termux-volume notification 2
elif [ "$ssid" = "Home-WiFi" ]; then
    # At home - normal
    termux-volume notification 7
fi
```

## 🎯 Productivity Tools

### Pomodoro Timer
```bash
#!/bin/bash
# pomodoro.sh - Pomodoro technique timer with notifications

WORK_DURATION=1500    # 25 minutes
BREAK_DURATION=300    # 5 minutes
LONG_BREAK=900        # 15 minutes

cycle=0

while true; do
    # Work session
    termux-notification -t "🍅 Work Time!" -c "Focus for 25 minutes"
    termux-tts-speak "Work time started"
    sleep $WORK_DURATION

    cycle=$((cycle + 1))

    # Break time
    if [ $((cycle % 4)) -eq 0 ]; then
        # Long break
        termux-notification -t "☕ Long Break!" -c "Take a 15 minute break" --vibrate "500,200,500"
        termux-tts-speak "Take a long break"
        sleep $LONG_BREAK
    else
        # Short break
        termux-notification -t "☕ Break Time!" -c "Take a 5 minute break" --vibrate "300,100,300"
        termux-tts-speak "Take a short break"
        sleep $BREAK_DURATION
    fi
done
```

### Clipboard History
```bash
#!/bin/bash
# clipboard_history.sh - Maintain clipboard history

HISTORY_FILE=~/clipboard_history.txt
MAX_ENTRIES=100

while true; do
    current=$(termux-clipboard-get)
    last=$(tail -n 1 $HISTORY_FILE)

    # If clipboard changed
    if [ "$current" != "$last" ] && [ ! -z "$current" ]; then
        timestamp=$(date +"%Y-%m-%d %H:%M:%S")
        echo "[$timestamp] $current" >> $HISTORY_FILE

        # Keep only last N entries
        tail -n $MAX_ENTRIES $HISTORY_FILE > ${HISTORY_FILE}.tmp
        mv ${HISTORY_FILE}.tmp $HISTORY_FILE
    fi

    sleep 2
done
```

### Task Reminder
```bash
#!/bin/bash
# task_reminder.sh - Location-based task reminders

TASKS_FILE=~/tasks.json

# Example tasks.json:
# [
#   {"task": "Buy groceries", "location": "Supermarket WiFi", "done": false},
#   {"task": "Pick up package", "location": "Post Office WiFi", "done": false}
# ]

while true; do
    ssid=$(termux-wifi-connectioninfo | jq -r '.ssid')

    # Check for tasks at this location
    tasks=$(jq --arg ssid "$ssid" '.[] | select(.location == $ssid and .done == false)' $TASKS_FILE)

    if [ ! -z "$tasks" ]; then
        task_name=$(echo $tasks | jq -r '.task')

        termux-notification \
            -t "📝 Task Reminder" \
            -c "Don't forget: $task_name" \
            --priority high \
            --vibrate "200,100,200,100,200"

        termux-tts-speak "Reminder: $task_name"
    fi

    sleep 60
done
```

## 🎮 Fun & Creative

### Time-Lapse Photography
```bash
#!/bin/bash
# timelapse.sh - Create time-lapse video

OUTPUT_DIR=~/timelapse/$(date +%Y%m%d_%H%M%S)
INTERVAL=60  # seconds
COUNT=100    # photos

mkdir -p $OUTPUT_DIR

for i in $(seq 1 $COUNT); do
    filename=$(printf "%04d.jpg" $i)
    termux-camera-photo -c 0 $OUTPUT_DIR/$filename

    termux-notification -t "📸 Time-lapse" -c "Photo $i/$COUNT captured"

    sleep $INTERVAL
done

termux-notification -t "✅ Time-lapse Complete" -c "$COUNT photos saved"

# Convert to video (requires ffmpeg)
# cd $OUTPUT_DIR && ffmpeg -framerate 30 -pattern_type glob -i '*.jpg' output.mp4
```

### Voice Note Journal
```bash
#!/bin/bash
# voice_journal.sh - Voice-based daily journal

JOURNAL_DIR=~/journal
mkdir -p $JOURNAL_DIR

termux-tts-speak "Ready to record your journal entry"
termux-notification -t "🎙️ Recording..." -c "Speak now"

# Record voice
text=$(termux-speech-to-text)

# Save with timestamp
timestamp=$(date +"%Y-%m-%d %H:%M:%S")
echo "[$timestamp] $text" >> $JOURNAL_DIR/$(date +%Y-%m).txt

termux-notification -t "✅ Journal Saved" -c "Entry recorded"
termux-tts-speak "Journal entry saved"
```

## 🔧 System Utilities

### Device Health Monitor
```bash
#!/bin/bash
# health_monitor.sh - Comprehensive device monitoring

while true; do
    # Battery
    battery=$(termux-battery-status)
    bat_percent=$(echo $battery | jq '.percentage')
    bat_temp=$(echo $battery | jq '.temperature')

    # Storage
    storage=$(df -h /sdcard | tail -1 | awk '{print $5}' | sed 's/%//')

    # WiFi
    wifi=$(termux-wifi-connectioninfo)
    wifi_speed=$(echo $wifi | jq '.link_speed')

    # Create health report
    report="Battery: ${bat_percent}% (${bat_temp}°C)\nStorage: ${storage}% used\nWiFi: ${wifi_speed} Mbps"

    # Check for issues
    issues=""
    [ $bat_percent -lt 20 ] && issues="$issues\n- Low battery"
    [ $bat_temp -gt 40 ] && issues="$issues\n- High temperature"
    [ $storage -gt 90 ] && issues="$issues\n- Low storage"

    if [ ! -z "$issues" ]; then
        termux-notification \
            -t "⚠️ Device Health Alert" \
            -c "Issues detected:$issues" \
            --priority high
    fi

    # Log report
    echo "$(date): $report" >> ~/logs/health.log

    sleep 1800  # Check every 30 minutes
done
```

---

## 📚 Integration with Pepebot

### Example: Use Pepebot with Termux API
```bash
# Let Pepebot control Android features

# 1. Battery check via Pepebot
~/pepebot agent -m "Check my battery status using termux-battery-status command"

# 2. Take photo via Pepebot
~/pepebot agent -m "Take a photo with the back camera and save to ~/photo.jpg"

# 3. Send notification via Pepebot
~/pepebot agent -m "Send me a notification saying 'Task completed'"

# 4. Get location via Pepebot
~/pepebot agent -m "What's my current location using GPS?"

# 5. Read clipboard via Pepebot
~/pepebot agent -m "What's in my clipboard?"
```

---

**Note:** All examples require appropriate Android permissions. Grant them in:
Settings → Apps → Termux:API → Permissions

Happy automating! 🐸📱
