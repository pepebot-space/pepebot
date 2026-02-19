import express from 'express';
import cors from 'cors';
import multer from 'multer';
import path from 'path';
import fs from 'fs';
import { fileURLToPath } from 'url';
import dotenv from 'dotenv';

dotenv.config();

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const app = express();
const PORT = process.env.PORT || 3000;
const isProduction = process.env.NODE_ENV === 'production';

// Middleware
app.use(cors());
app.use(express.json());
app.use('/uploads', express.static(path.join(__dirname, 'uploads')));

// Storage for uploaded files
const uploadDir = path.join(__dirname, 'uploads');
if (!fs.existsSync(uploadDir)) {
    fs.mkdirSync(uploadDir, { recursive: true });
}

const storage = multer.diskStorage({
    destination: (req, file, cb) => cb(null, uploadDir),
    filename: (req, file, cb) => {
        const uniqueSuffix = Date.now() + '-' + Math.round(Math.random() * 1E9);
        cb(null, uniqueSuffix + '-' + file.originalname);
    }
});
const upload = multer({ storage });

// ─── API Routes ─────────────────────────────────────────────

// Health Check
app.get('/api/health', (req, res) => {
    res.json({ status: 'ok', timestamp: new Date() });
});

// Upload Image
app.post('/api/upload', upload.single('image'), (req, res) => {
    if (!req.file) {
        return res.status(400).json({ error: 'No file uploaded' });
    }
    const fileUrl = `${req.protocol}://${req.get('host')}/uploads/${req.file.filename}`;
    res.json({ success: true, url: fileUrl, filename: req.file.filename });
});

// ─── Server Startup ─────────────────────────────────────────

async function startServer() {
    if (isProduction) {
        // Production: serve pre-built static files
        const distPath = path.join(__dirname, 'client', 'dist');
        app.use(express.static(distPath));

        // SPA fallback
        app.get(/(.*)/, (req, res) => {
            res.sendFile(path.join(distPath, 'index.html'));
        });

        console.log(`📦 Serving static files from: ${distPath}`);
    } else {
        // Development: use Vite dev server as middleware
        const { createServer: createViteServer } = await import('vite');

        const vite = await createViteServer({
            configFile: path.join(__dirname, 'client', 'vite.config.js'),
            root: path.join(__dirname, 'client'),
            server: { middlewareMode: true },
            appType: 'spa',
        });

        app.use(vite.middlewares);

        console.log('⚡ Vite dev server attached as middleware');
    }

    app.listen(PORT, () => {
        const mode = isProduction ? 'Production' : 'Development';
        console.log(`
╔═══════════════════════════════════════════╗
║  Pepebot Dashboard                        ║
║  URL: http://localhost:${String(PORT).padEnd(5)}              ║
║  Mode: ${mode.padEnd(15)}                   ║
╚═══════════════════════════════════════════╝
        `);
    });
}

startServer().catch((error) => {
    console.error('Failed to start server:', error);
    process.exit(1);
});
