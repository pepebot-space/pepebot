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

# Hint aktivasi mode video pada sesi live
# Dukungan video explicit: vertex, gemini
PEPEBOT_LIVE_VIDEO=false

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
- **`session_key`** *(string)*: Nama percakapan. Ini batas memori — lihat [Sesi per device](#sesi-per-device) di bawah.
- **`enable_tools`** *(boolean)*: Set ke `true` jika Anda menghendaki agen di dalam percakapan suara ini diizinkan untuk memanggil ekstensi tools (misalnya web search, scraping, dll).

> Tools bekerja untuk provider Vertex/Gemini maupun provider dengan protokol OpenAI Realtime (`openai`, `maiarouter`, `realtime`). Untuk protokol Realtime, definisi tool dikirim lewat `session.update`, dan panggilan tool dibaca dari `response.output_item.done` (`item.type == "function_call"`) lalu hasilnya dibalas sebagai item `function_call_output` diikuti `response.create`.

### Tools milik aplikasi klien

Aplikasi yang terhubung bisa mendaftarkan tool-nya sendiri dan mengeksekusinya di
device-nya sendiri — kamera, GPS, layar, sensor, apa pun yang gateway tidak punya.
Deklarasikan di setup, dengan `app` sebagai namespace-nya:

```json
{
  "setup": {
    "provider": "realtime",
    "app": "rover",
    "client_tool_timeout_ms": 30000,
    "tools": [
      {
        "name": "take_photo",
        "description": "Take a photo with the device camera.",
        "parameters": {"type": "object", "properties": {
          "camera": {"type": "string", "enum": ["front", "back"]}}}
      }
    ]
  }
}
```

Model melihatnya sebagai `rover-take_photo`; klien ditanya dengan nama aslinya
(`take_photo`). Tool gateway semuanya `snake_case`, jadi tanda hubung membuat tool
klien tidak mungkin menabrak tool gateway. Nama `app` dan nama tool harus cocok
`^[A-Za-z0-9_]{1,48}$` — deklarasi yang salah ditolak saat setup dengan pesan yang
menyebut masalahnya, bukan didiamkan.

Saat model memanggilnya, Pepebot mengirim ke klien:

```json
{"type": "tool_call", "call_id": "call_abc", "name": "take_photo",
 "arguments": {"camera": "back"}}
```

Klien menjawab dengan salah satu dari:

```json
{"type": "tool_result", "call_id": "call_abc", "output": "seekor kucing di keyboard"}
{"type": "tool_result", "call_id": "call_abc", "error": "izin kamera ditolak"}
```

`error` sampai ke model sebagai `Error: <pesan>` — bentuk yang sama dengan tool gateway
yang gagal, jadi model bisa menyebutkannya alih-alih menggantung. Frame `tool_result`
tidak diteruskan ke upstream; upstream hanya melihat `function_call_output` yang
dibentuk Pepebot darinya.

Klien yang lambat, ter-background, atau hilang dibatasi
`client_tool_timeout_ms` (default 30 detik, maksimum 90 detik). Lewat batas itu model
diberi tahu `Error: client tool timed out` dan turn tetap selesai.

Contoh lengkap ada di `examples/live-api/paniki.html`, yang mendaftarkan
`device_info` dan `geolocate`.

### Sesi per device

`setup.session_key` menamai percakapan, dan **itu batas memorinya**. Setiap device kirim
key-nya sendiri, maka masing-masing punya percakapan sendiri:

```json
{"setup": {"provider": "realtime", "session_key": "rover-01"}}
{"setup": {"provider": "realtime", "session_key": "hp-ibnu"}}
{"setup": {"provider": "realtime", "session_key": "kios-lobi-3"}}
```

Yang terjadi dengan key itu:

- **Turn disimpan** ke session history agent di bawah nama itu
- **Dan diputar ulang** (20 turn terakhir) saat ada sesi baru dengan nama yang sama —
  jadi percakapan bertahan melewati reconnect, ganti client, bahkan restart gateway
- **Key yang sama = satu percakapan**, meski beda transport. Sesi suara dan chat teks
  dengan key sama saling ingat
- **Key berbeda = percakapan berbeda**, tidak saling tahu apa pun

Terukur:

```
[rover-01] "Ingat angka favoritku 77."   -> "Siap, sudah kucatat."
[rover-01] KONEKSI BARU, key sama        -> "Angka favoritmu 77."
[hp-ibnu]  pertanyaan sama               -> "Aku tidak tahu."
```

Kalau `session_key` tidak dikirim, Pepebot membuat key sekali-pakai
(`live:<provider>:<agent>:<timestamp>`) — sesi jalan normal tapi tidak ada yang bisa
melanjutkannya. Untuk device tetap, kirim key yang stabil: id device, nomor seri, atau
nomor telepon.

Dua puluh turn itu jendela, bukan arsip — server upstream menagih setiap token yang
dikirim balik.

Di client yang ada: `SESSION=nama` untuk client terminal, dan field **Session** di
`paniki.html` (tampil di header, diingat per browser, jadi dua tab bisa jadi dua
percakapan). Rinciannya: [examples/live-api/README-realtime.md](../examples/live-api/README-realtime.md#chat-sessions).

### Gambar dan video di sesi Realtime

Klien bisa mengirim gambar ke model — proxy meneruskan frame klien apa adanya, jadi
tidak ada perubahan di sisi Pepebot. Hanya satu bentuk yang benar-benar dibaca:

```json
{"type":"conversation.item.create","item":{"type":"message","role":"user","content":[
  {"type":"input_image","image_url":"data:image/png;base64,..."},
  {"type":"input_text","text":"Ini warna apa?"}]}}
```

Empat bentuk block dibaca — `input_image`+`image_url`, `image_url`+`image_url.url`,
`image`+`source.base64` (gaya Anthropic), dan `input_image`+`data`. Yang salah
menghasilkan error, bukan diabaikan: part tanpa gambar terbaca → `unreadable_image`,
URL remote → `remote_image` (ditolak, tidak di-fetch, supaya server ini tidak jadi alat
SSRF).

Untuk "video": tidak ada event streaming, dan tidak perlu — **hanya gambar terbaru yang
disimpan**, jadi frame yang dikirim sebagai item dikonsumsi, bukan menumpuk. 30 frame
merah lalu satu biru dijawab "Biru", dan tidak lebih lambat dari satu frame.

Konsekuensinya: model hanya bisa melihat satu gambar sekaligus, jadi "bandingkan dua
gambar ini" tidak jalan kecuali keduanya ditaruh dalam satu item.

Rinciannya, termasuk cara mem-probe server Realtime lain: **[docs/live-vision.md](./live-vision.md)**.

### Konfigurasi `live.realtime_session`

Field apa pun yang diizinkan server Realtime upstream (misalnya `voice`, `temperature`, `max_response_output_tokens`, `turn_detection`) bisa diisi di sini dan akan digabungkan ke `session.update`. `instructions` dan `tools` milik Pepebot selalu menang jika ada key yang sama.

```json
"live": {
  "provider": "realtime",
  "realtime_session": { "voice": "JV-00027", "temperature": 0.2, "max_response_output_tokens": 200 }
}
```

### Konfigurasi `live.video`

Anda bisa mengaktifkan hint mode video langsung dari config:

```json
{
  "live": {
    "video": true
  }
}
```

Saat koneksi `/v1/live` berhasil, server akan mengirim metadata video di pesan `connected`:
- `video.requested`: nilai dari config (`live.video`)
- `video.supported`: apakah provider sesi saat ini punya dukungan video explicit
- `video.enabled`: `true` jika `requested && supported`

Untuk provider `vertex` dan `gemini`, `live.video=true` digunakan sebagai capability flag agar client dapat mengaktifkan stream kamera.
`generationConfig.responseModalities` tetap mengikuti konfigurasi model yang Anda pakai.

Provider dengan dukungan video explicit saat ini:
- `vertex`
- `gemini`

Jika koneksi dan inisialisasi berhasil, Pepebot Server akan membalas dengan status konfirmasi, misalnya:
- Untuk Vertex/Gemini: `{"setupComplete": true}` atau respon ekuivalen yang menandakan Live session siap.
- Untuk OpenAI / MAIA Router: `{"status": "connected", "provider": "openai", "model": "gpt-4o-realtime-preview", "session": "..."}`. Setelah menerima respons ini, klien dapat mulai mengirimkan *frame* mengikuti skema spesifik dari provider tersebut.

### Tuning Latency Video (kapan model "melihat" gambar)

Pada Vertex/Gemini, frame kamera dikirim sebagai `realtimeInput` (streaming kontinu). Model **tidak** memproses tiap frame seketika — ia hanya "melihat" frame terbaru saat *turn* di-commit, yaitu ketika *Voice Activity Detection* (VAD) mendeteksi Anda berhenti bicara. Jadi delay yang terasa sebagian besar berasal dari konfigurasi turn-taking, bukan dari proxy Pepebot (yang transparan).

Tiga knob yang memengaruhi delay (semua opsional, punya default yang baik):

```json
{
  "live": {
    "media_resolution": "MEDIA_RESOLUTION_LOW",
    "generation_config": {},
    "realtime_input_config": {
      "automaticActivityDetection": {
        "disabled": false,
        "startOfSpeechSensitivity": "START_SENSITIVITY_LOW",
        "endOfSpeechSensitivity": "END_SENSITIVITY_HIGH",
        "prefixPaddingMs": 80,
        "silenceDurationMs": 500
      }
    }
  }
}
```

| Field | Efek | Default Pepebot |
|-------|------|-----------------|
| `media_resolution` | `MEDIA_RESOLUTION_LOW` ≈ 280 token/gambar → inferensi lebih cepat & murah; `MEDIUM/HIGH` lebih detail tapi lebih lambat. Disuntikkan otomatis ke `generationConfig.mediaResolution` bila belum di-set. | `MEDIA_RESOLUTION_LOW` |
| `endOfSpeechSensitivity` | `END_SENSITIVITY_HIGH` = lebih cepat memutuskan ucapan selesai → turn cepat commit → gambar cepat diproses. `LOW` menunggu lebih lama. | `END_SENSITIVITY_HIGH` |
| `silenceDurationMs` | Lama hening sebelum turn dianggap selesai. Makin kecil makin responsif, tapi terlalu kecil bisa memotong ucapan. | `500` |

> ⚠️ Jika `realtime_input_config` Anda set eksplisit di `config.json`, isinya **menggantikan** default (bukan merge). Pastikan menyertakan `endOfSpeechSensitivity`/`silenceDurationMs` yang diinginkan.

**Snapshot (lihat sekarang juga):** untuk memaksa model melihat satu frame seketika tanpa menunggu VAD, kirim frame sebagai `clientContent` dengan `turnComplete: true` (bukan `realtimeInput`). Contoh di `examples/live-api/index-video.html` punya tombol **Snapshot**:

```json
{
  "clientContent": {
    "turns": [{ "role": "user", "parts": [
      { "inlineData": { "mimeType": "image/jpeg", "data": "<base64 jpeg>" } },
      { "text": "apa yang saya pegang?" }
    ]}],
    "turnComplete": true
  }
}
```

### System Prompt (`systemInstruction`)

Anda bisa memberi peran/persona/instruksi tugas ke model Live (Vertex/Gemini) lewat `systemInstruction`. Pepebot menyusunnya dengan urutan presedensi:

1. **Per-sesi (paling tinggi)** — field `system_prompt` di pesan `setup` dari klien.
2. **Config default** — `live.system_prompt` (atau `live.system_prompt_file`; env `PEPEBOT_LIVE_SYSTEM_PROMPT`).
3. **Persona agent (opsional)** — bila `live.use_agent_prompt: true`, fallback ke file persona agent terpilih (`AGENTS.md` → `SOUL.md` → `IDENTITY.md`, cek di direktori agent lalu workspace).

Jika tidak ada satu pun yang di-set, perilaku **identik** seperti sebelumnya (upstream setup tanpa `systemInstruction`, tanpa regresi).

**Config default:**

```json
{
  "live": {
    "enabled": true,
    "provider": "vertex",
    "model": "gemini-live-2.5-flash-native-audio",
    "video": true,
    "system_prompt": "You are LEXA, an autonomous rover. You can see the camera and call rover tools. Given a goal, work toward it in a perceive→act→perceive loop with small bounded steps; avoid obstacles (back off + turn on blocked); stop immediately when asked. Narrate briefly."
  }
}
```

- Alternatif dari file: `"system_prompt_file": "~/.pepebot/lexa.md"` (dipakai hanya bila `system_prompt` kosong; mendukung `~`).
- Pakai persona agent: `"use_agent_prompt": true` (hanya berlaku jika `system_prompt`/`system_prompt_file` kosong).

**Override per-sesi (dari klien):**

```json
{ "setup": { "provider": "vertex", "model": "gemini-live-2.5-flash-native-audio",
             "agent": "default", "enable_tools": true,
             "system_prompt": "You are LEXA, an autonomous rover ..." } }
```

Hasilnya, `BidiGenerateContentSetup` upstream akan menyertakan:

```json
{ "setup": { "systemInstruction": { "parts": [ { "text": "<PROMPT>" } ] }, "model": "...", "generationConfig": {} } }
```

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


> 💡 **Tip Implementasi Klien:** Anda dapat melihat kode lengkap (*source code*) integrasi Web HTML, Python, dan ekosistem terkait Live API ini di bagian direktori [examples/live-api/](../examples/live-api/) termasuk contoh video `index-video.html` dan `client-video.py`.

Untuk varian OpenAI Realtime event protocol, gunakan contoh:
- `examples/live-api/paniki.html` (khusus server Paniki: model diambil dari `/v1/models`, tool call ditampilkan, voice discovery)
- `examples/live-api/index-openai.html` (generic OpenAI Realtime; provider/model/gateway editable)
- `examples/live-api/README-realtime.md` (how to point it at a custom Realtime endpoint)
- `examples/live-api/client-openai.py`
