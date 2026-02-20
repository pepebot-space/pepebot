// Gateway configuration manager
// Stores gateway server configs in localStorage

const STORAGE_KEY = 'pepebot_gateways'
const ACTIVE_KEY = 'pepebot_active_gateway'

const FROG_NAMES = [
    'Kermit', 'Pepe', 'Ribbit', 'Froggy', 'Jumper', 'Croaky',
    'Gecko', 'Toad', 'Bullfrog', 'TreeFrog', 'PoisonDart',
    'GreenBean', 'LilyPad', 'Splash', 'Hopper', 'Swampy'
]

function randomName() {
    const name = FROG_NAMES[Math.floor(Math.random() * FROG_NAMES.length)]
    const num = Math.floor(Math.random() * 1000)
    return `${name}-${num}`
}

export function getGateways() {
    try {
        return JSON.parse(localStorage.getItem(STORAGE_KEY) || '[]')
    } catch {
        return []
    }
}

export function saveGateways(gateways) {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(gateways))
}

export function addGateway({ name, url }) {
    const gateways = getGateways()
    const id = Date.now().toString(36)
    const gw = {
        id,
        name: name?.trim() || randomName(),
        url: url.replace(/\/+$/, ''), // remove trailing slash
        createdAt: new Date().toISOString()
    }
    gateways.push(gw)
    saveGateways(gateways)
    return gw
}

export function updateGateway(id, updates) {
    const gateways = getGateways()
    const idx = gateways.findIndex(g => g.id === id)
    if (idx !== -1) {
        gateways[idx] = { ...gateways[idx], ...updates }
        saveGateways(gateways)
    }
    return gateways[idx]
}

export function removeGateway(id) {
    const gateways = getGateways().filter(g => g.id !== id)
    saveGateways(gateways)
    // If active was removed, clear it
    if (getActiveGatewayId() === id) {
        setActiveGateway(null)
    }
}

export function getActiveGatewayId() {
    return localStorage.getItem(ACTIVE_KEY) || null
}

export function setActiveGateway(id) {
    if (id) {
        localStorage.setItem(ACTIVE_KEY, id)
    } else {
        localStorage.removeItem(ACTIVE_KEY)
    }
}

export function getActiveGateway() {
    const id = getActiveGatewayId()
    if (!id) return null
    const gateways = getGateways()
    return gateways.find(g => g.id === id) || null
}

export function getGatewayApiUrl() {
    const gw = getActiveGateway()
    if (!gw) return null
    // Ensure URL ends with /v1
    const base = gw.url.replace(/\/+$/, '')
    return base.endsWith('/v1') ? base : `${base}/v1`
}

export function isGatewayConfigured() {
    return !!getActiveGateway()
}
