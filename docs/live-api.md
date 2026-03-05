# Panduan Konfigurasi Live API (Voice Agent)

Live API di Pepebot memungkinkan interaksi suara secara *real-time streaming* dengan latency yang sangat rendah menggunakan koneksi WebSocket. Fitur ini dirancang agar Pepebot dapat "berbicara" langsung secara dua arah. Saat ini, Live API mendukung berbagai provider seperti Vertex AI, OpenAI, Gemini, dan MaiaRouter.

## 1. Konfigurasi Server (Environment Variables)

Untuk mengaktifkan dan mengonfigurasi Live API pada server Pepebot, Anda perlu menyesuaikan file `.env` di sistem Anda. Berikut adalah *environment variables* yang digunakan:

```env
# Mengaktifkan endpoint WebSocket Live API di /v1/live
PEPEBOT_LIVE_ENABLED=true

# Provider utama yang akan digunakan untuk koneksi Live API
# Pilihan yang didukung: vertex, openai, gemini, maiarouter
PEPEBOT_LIVE_PROVIDER=vertex

# Model default untuk sesi Live API
# Contoh Model Vertex/Gemini: gemini-live-2.5-flash-native-audio
# Contoh Model OpenAI: gpt-4o-realtime-preview
PEPEBOT_LIVE_MODEL=gemini-live-2.5-flash-native-audio

# Bahasa default untuk Live API (opsional)
PEPEBOT_LIVE_LANGUAGE=id-ID
```

> **Catatan:** Selain variable Live di atas, pastikan Anda juga sudah mengonfigurasi kredensial (API key) dari provider yang Anda pilih. Misalnya mengatur `PEPEBOT_PROVIDERS_VERTEX_CREDENTIALS_FILE` jika Anda menggunakan `vertex`.

---

## 2. Endpoint WebSocket Utama

Setelah Pepebot dijalankan (Gateway Service *running*), endpoint WebSocket untuk mengakses fitur Live API berada di dalam port Gateway (default: `18790`).

**URL Endpoint:**
```
ws://<HOST>:18790/v1/live
```
*(Ubah `<HOST>` dari `127.0.0.1` ke IP yang sesuai apabila mengakses dari luar server lokal)*

---

## 3. Konfigurasi Setup Sesi Klien (Client Setup)

Saat pertama kali klien terhubung ke WebSocket API, klien **diwajibkan** untuk mengirim pesan JSON yang berisi metadata/pengaturan awal (*setup payload*).

Fitur unggulan di Live API adalah **Integrasi Agent**, di mana alur *real-time voice* ini bisa mewarisi sistem prompt (instruksi persona), tools, dan buffer obrolan dari konfigurasi agen biasa di Pepebot Workspace Anda.

**Contoh Payload Setup (Format JSON):**

*A. Untuk Koneksi Vertex/Gemini Realtime:*
```json
{
  "setup": {
    "provider": "vertex",
    "model": "gemini-live-2.5-flash-native-audio",
    "agent": "default",
    "session_key": "unique-client-session-id-1234",
    "enable_tools": true
  }
}
```

*B. Untuk Koneksi OpenAI Realtime API (atau MAIA Router):*
```json
{
  "setup": {
    "provider": "openai",
    "model": "gpt-4o-realtime-preview",
    "agent": "default",
    "session_key": "unique-client-session-id-1234",
    "enable_tools": true
  }
}
```

### Penjelasan *Field* Setup:
- **`provider`** *(string)*: Provider AI yang akan digunakan. Bisa mengikuti env `PEPEBOT_LIVE_PROVIDER`.
- **`model`** *(string)*: Model AI *real-time* yang spesifik dipakai oleh sesi.
- **`agent`** *(string)*: Nama agen dari workspace tempat instruksi / persona disimpan (misalnya `default`, atau nama file spesifik agen Anda). Pepebot secara otomatis akan menarik *system prompt* agen ini dan menyuntikkannya ke dalam instruksi *upstream* API suara.
- **`session_key`** *(string)*: Kunci unik untuk *state* sesi. Digunakan Pepebot unuk memanjangkan obrolan *(Chat History)* pada sesi yang berlanjut.
- **`enable_tools`** *(boolean)*: Set ke `true` jika Anda menghendaki agen di dalam percakapan suara ini diizinkan untuk memanggil ekstensi tools (misalnya web search, scraping, dll).

Jika koneksi dan inisialisasi berhasil, Pepebot Server akan membalas dengan status konfirmasi, misalnya:
- Untuk Vertex/Gemini: `{"setupComplete": true}` atau respon ekuivalen yang menandakan Live session siap.
- Untuk OpenAI / MAIA Router: `{"status": "connected", "provider": "openai", "model": "gpt-4o-realtime-preview", "session": "..."}`. Setelah menerima respons ini, klien dapat mulai mengirimkan *frame* mengikuti skema spesifik dari provider tersebut.

---

## 4. Pengiriman Audio dan Teks

Secara umum, format interaksi berbasis *frame* JSON. Karena Pepebot Gateway bertindak sebagai jalur *proxy* *real-time* ganda (*transparent bidirectional proxy*), struktur pesan JSON (`payload`) yang harus dikirim oleh klien sangat bergantung pada **provider AI** yang digunakan.

### A. Provider Vertex AI / Gemini

- **Mengirim Audio (Klien ke Server):**
  Untuk Model tipe Vertex/Gemini Realtime secara umum mengharapkan file audio mentah direkam di *input sample rate* 16000Hz PCM, kemudian diselipkan dalam bentuk Base64 ke frame JSON:
  
  ```json
  {
    "realtimeInput": {
      "mediaChunks": [
        {
          "mimeType": "audio/pcm;rate=16000",
          "data": "base_64_encoded_pcm_frame_here..."
        }
      ]
    }
  }
  ```

- **Menerima Audio (Server ke Klien):**
  Respons dibungkus dengan format serupa (`serverContent`). Di dalamnya akan terdapat obrolan teks maupun `inlineData` Base64 yang akan di-*decode* oleh sisi klien pada *output sample rate* 24000Hz (sebagai contoh).

### B. Provider OpenAI (Realtime API)

Untuk provider OpenAI (serta proxy MAIA Router yang kompatibel dengan protokol OpenAI), klien menggunakan format *event* WebSockets standar [OpenAI Realtime API](https://developers.openai.com/api/reference/resources/realtime/).

- **Mengirim Audio (Klien ke Server):**
  Setelah terhubung, klien menyuntikkan audio lokal menggunakan *event* tipe `input_audio_buffer.append` berupa audio *base64* PCM16 di *sample rate* 24kHz:
  
  ```json
  {
    "type": "input_audio_buffer.append",
    "audio": "base_64_encoded_pcm_frame_here..."
  }
  ```

- **Menerima Audio (Server ke Klien):**
  Server akan merespons dalam berbagai bentuk *event*, misalnya teks, transkrip, maupun terpenting `response.audio.delta` yang membawa respons suara agen untuk diputar di sisi klien:

  ```json
  {
    "type": "response.audio.delta",
    "response_id": "...",
    "item_id": "...",
    "output_index": 0,
    "content_index": 0,
    "delta": "Base64 encoded audio data..."
  }
  ```
  *(Pastikan klien Anda memonitor event stream lain seperti `session.update`, `response.create`, dll sesuai dokumentasi resmi OpenAI Realtime).*


> 💡 **Tip Implementasi Klien:** Anda dapat melihat kode lengkap (*source code*) integrasi Web HTML, Python, dan ekosistem terkait Live API ini di bagian direktori [examples/live-api/](../examples/live-api/).

