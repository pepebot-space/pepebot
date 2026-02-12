# Termux API Skill 📱

Control your Android device hardware and system features through Termux API commands.

## 🎯 What This Skill Does

This skill enables Pepebot to interact with Android device features when running on Android via Termux:

- 🔋 **Battery & Power**: Check battery status, temperature, charging state
- 📸 **Camera**: Take photos with front/back camera
- 📋 **Clipboard**: Read/write system clipboard
- 📞 **Contacts**: Access contact list
- 💬 **Dialogs**: Show user input dialogs
- 📍 **Location**: Get GPS/network location
- 🔊 **Media**: Play audio, text-to-speech, speech-to-text
- 🔔 **Notifications**: Display system notifications
- 📱 **Sensors**: Access accelerometer, gyroscope, light sensor, etc.
- 📞 **SMS/Phone**: Send SMS, make calls
- 💾 **Storage**: Access shared storage
- 🌐 **WiFi**: Get connection info, scan networks
- 🎛️ **System**: Control brightness, volume, vibration

## 📦 Installation

### 1. Install Termux API App
Download from **F-Droid** (NOT Google Play):
https://f-droid.org/packages/com.termux.api/

### 2. Install Termux API Package
```bash
pkg install termux-api
```

### 3. Grant Permissions
When you run a command for the first time, Android will ask for permissions. Grant them or go to:
**Settings → Apps → Termux:API → Permissions**

### 4. Test Installation
```bash
termux-battery-status
```

If you see battery info JSON, it's working! ✅

## 🚀 Quick Start

### Example Commands

```bash
# Check battery
termux-battery-status

# Take photo
termux-camera-photo -c 0 ~/photo.jpg

# Get clipboard
termux-clipboard-get

# Send notification
termux-notification -t "Hello" -c "From Pepebot"

# Get location
termux-location -p network -r once

# Text to speech
termux-tts-speak "Hello from Pepebot"

# Get WiFi info
termux-wifi-connectioninfo
```

### Use with Pepebot

```bash
# Start Pepebot
~/pepebot agent

# Then ask:
> Check my battery status
> Take a photo with the back camera
> What's my current location?
> Send me a notification saying task is done
> Read my clipboard
```

## 📚 Documentation

- **[SKILL.md](SKILL.md)** - Complete API reference with all commands
- **[EXAMPLES.md](EXAMPLES.md)** - Real-world use cases and automation scripts

## 🎯 Use Cases

### Personal Assistant
- Voice commands with speech-to-text
- Voice responses with text-to-speech
- Smart notifications based on location/time

### Home Automation
- Location-based actions (arrive/leave home)
- WiFi-based scene switching
- Battery-aware automation

### Security & Monitoring
- Motion detection alerts
- Location tracking
- SMS security notifications
- Photo capture on motion

### Productivity
- Pomodoro timer with notifications
- Clipboard history manager
- Location-based task reminders
- Voice note journal

### Data Collection
- Battery life analytics
- Location history logging
- WiFi network mapping
- Sensor data collection

## 🔐 Required Permissions

The skill will request these Android permissions as needed:

| Feature | Permission |
|---------|-----------|
| Camera | Camera |
| Location | Location (Coarse/Fine) |
| Microphone | Microphone |
| Contacts | Read Contacts |
| SMS | Send/Read SMS |
| Phone | Make Calls, Read Phone State |
| Storage | Read/Write External Storage |
| Sensors | Body Sensors |

## 💡 Tips

1. **Background Execution**: Use `tmux` to keep scripts running:
   ```bash
   tmux new -s bot
   ~/pepebot gateway
   # Detach: Ctrl+B then D
   ```

2. **Auto-start on Boot**: Use Termux:Boot app
   ```bash
   mkdir -p ~/.termux/boot
   echo '~/pepebot gateway &' > ~/.termux/boot/start.sh
   chmod +x ~/.termux/boot/start.sh
   ```

3. **Disable Battery Optimization**:
   Settings → Apps → Termux → Battery → Unrestricted

4. **JSON Parsing**: Install `jq` for JSON parsing:
   ```bash
   pkg install jq
   ```

## 🐛 Troubleshooting

**"Command not found"**
```bash
pkg update
pkg install termux-api
```

**"Permission denied"**
- Grant permissions in Android Settings
- Restart Termux after granting permissions

**"No camera available"**
- Close all apps using camera
- Try different camera ID (0 or 1)

**"Location not available"**
- Enable Location in Android Quick Settings
- Grant location permission to Termux:API
- Try different provider: `gps` or `network`

## 🔗 Links

- [Termux Wiki](https://wiki.termux.com/wiki/Termux:API)
- [Termux API GitHub](https://github.com/termux/termux-api)
- [Pepebot Docs](../../README.md)
- [Android Setup Guide](../../ANDROID.md)

## 📝 License

This skill is part of Pepebot and follows the same MIT License.

---

**Platform Support**: Android only (via Termux)

Made with 🐸 by Pepebot Contributors
