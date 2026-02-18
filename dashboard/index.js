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

// 2. Proxy to Pepebot Gateway - Send Message
app.post('/api/chat', async (req, res) => {
    try {
        const { message, session_key } = req.body;
        // Build payload for Pepebot
        const payload = {
            message,
            session_key: session_key || 'dashboard:default',
            channel: 'gateway'
        };

        const response = await axios.post(`${PEPEBOT_GATEWAY}/message`, payload);
        res.json(response.data);
    } catch (error) {
        console.error('Error Proxying to Pepebot:', error.message);
        if (error.code) console.error('Error Code:', error.code);
        if (error.response) console.error('Response Data:', error.response.data);
        res.status(500).json({ error: 'Failed to communicate with Pepebot Gateway', details: error.message || error.code });
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

// 4. Get Agents (Mock or Real if API exists)
app.get('/api/agents', async (req, res) => {
    // If Pepebot has an endpoints for agents, fetch it. Otherwise return mock for now.
    // Based on docs, /status returns agent info.
    try {
        const response = await axios.get(`${PEPEBOT_GATEWAY}/status`);
        res.json(response.data);
    } catch (error) {
        // Fallback if gateway is down
        res.json({
            agents: [
                { name: 'Pepebot', status: 'offline', version: '0.4.0' }
            ]
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
