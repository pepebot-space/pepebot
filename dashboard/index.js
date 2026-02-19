const express = require('express');
const cors = require('cors');
const multer = require('multer');
const path = require('path');
const axios = require('axios');
const { PrismaClient } = require('@prisma/client');
require('dotenv').config();

const app = express();
const prisma = new PrismaClient();
const PORT = process.env.PORT || 3000;
const PEPEBOT_GATEWAY = process.env.PEPEBOT_GATEWAY || 'http://localhost:18790';

// Middleware
app.use(cors());
app.use(express.json());
app.use('/uploads', express.static(path.join(__dirname, 'uploads')));
app.use(express.static(path.join(__dirname, './client/dist'))); // Serve frontend

// Storage for uploaded files
const storage = multer.diskStorage({
    destination: function (req, file, cb) {
        const uploadDir = path.join(__dirname, 'uploads');
        // Ensure uploads directory exists (should be created on startup ideally)
        const fs = require('fs');
        if (!fs.existsSync(uploadDir)) {
            fs.mkdirSync(uploadDir);
        }
        cb(null, uploadDir)
    },
    filename: function (req, file, cb) {
        const uniqueSuffix = Date.now() + '-' + Math.round(Math.random() * 1E9)
        cb(null, uniqueSuffix + '-' + file.originalname)
    }
})
const upload = multer({ storage: storage });

// Routes

// 1. Health Check
app.get('/api/health', (req, res) => {
    res.json({ status: 'ok', timestamp: new Date() });
});

// 2. Proxy to Pepebot Gateway - Send Message (OpenAI Compatible)
// 2. Proxy to Pepebot Gateway - Send Message (OpenAI Compatible with SSE)
app.post('/api/chat', async (req, res) => {
    try {
        const { message, session_key, media, agent_id } = req.body;

        // Construct messages array
        const messages = [{ role: "user", content: message }];
        if (media && media.length > 0) {
            messages[0].content = [
                { type: "text", text: message },
                ...media.map(url => ({ type: "image_url", image_url: { url } }))
            ];
        }

        const payload = {
            model: "maia/gemini-3-pro-preview",
            messages: messages,
            stream: true // Enable streaming
        };

        // Important: Axios needs responseType: 'stream' to handle SSE
        const response = await axios({
            method: 'post',
            url: `${PEPEBOT_GATEWAY}/v1/chat/completions`,
            data: payload,
            headers: {
                'Content-Type': 'application/json',
                'X-Session-Key': session_key || 'web:default',
                'X-Agent': agent_id || 'default'
            },
            responseType: 'stream'
        });

        // Set headers for SSE
        res.setHeader('Content-Type', 'text/event-stream');
        res.setHeader('Cache-Control', 'no-cache');
        res.setHeader('Connection', 'keep-alive');

        // Pipe the stream from the gateway to the client
        response.data.pipe(res);

    } catch (error) {
        console.error('Error Proxying to Pepebot:', error.message);
        // If headers aren't sent, send error JSON
        if (!res.headersSent) {
            res.status(500).json({ error: 'Failed to communicate with Pepebot Gateway', details: error.message });
        }
    }
});

// 2.1 Get Sessions
app.get('/api/sessions', async (req, res) => {
    try {
        const response = await axios.get(`${PEPEBOT_GATEWAY}/v1/sessions`);
        res.json(response.data);
    } catch (error) {
        res.json({ sessions: [] }); // Valid fallback
    }
});

// 2.2 Create New Session
app.post('/api/sessions/new', async (req, res) => {
    try {
        const { key } = req.body;
        // POST /v1/sessions/{key}/new
        const response = await axios.post(`${PEPEBOT_GATEWAY}/v1/sessions/${key}/new`);
        res.json(response.data);
    } catch (error) {
        console.error('Error creating session:', error.message);
        res.status(500).json({ error: 'Failed to create session' });
    }
});

// 3. Upload Image
app.post('/api/upload', upload.single('image'), (req, res) => {
    if (!req.file) {
        return res.status(400).json({ error: 'No file uploaded' });
    }
    // Construct full URL for the uploaded file
    const fileUrl = `${req.protocol}://${req.get('host')}/uploads/${req.file.filename}`;
    res.json({ success: true, url: fileUrl, filename: req.file.filename });
});

// 4. Get Agents
app.get('/api/agents', async (req, res) => {
    try {
        const response = await axios.get(`${PEPEBOT_GATEWAY}/v1/agents`);
        res.json(response.data);
    } catch (error) {
        // Fallback if gateway is down
        res.json({
            agents: {
                default: { name: 'Pepebot (Offline)', status: 'offline', version: '0.4.0' }
            }
        });
    }
});

// 5. Catch-all for SPA
app.get(/(.*)/, (req, res) => {
    res.sendFile(path.join(__dirname, './client/dist/index.html'));
});

app.listen(PORT, () => {
    console.log(`Server is running on http://localhost:${PORT}`);
});
