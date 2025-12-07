import { httpClient } from '../dataProvider'
import { baseUrl } from '../utils'

const API_BASE = '/api/ai-dj'

// Helper function for fetching audio (bypasses JSON parsing)
const fetchAudio = async (url) => {
  const token = localStorage.getItem('token')
  const headers = {}
  if (token) {
    headers['X-ND-Authorization'] = `Bearer ${token}`
  }

  const response = await fetch(baseUrl(url), { headers })
  if (!response.ok) {
    throw new Error(`TTS request failed: ${response.status}`)
  }
  return response.blob()
}

const aiDjApi = {
  startSession: async (
    mode,
    seedTrackId,
    seedArtistId,
    seedPlaylistId,
    preferences,
  ) => {
    const body = {}
    if (mode) body.mode = mode
    if (seedTrackId) body.seedTrackId = seedTrackId
    if (seedArtistId) body.seedArtistId = seedArtistId
    if (seedPlaylistId) body.seedPlaylistId = seedPlaylistId
    if (preferences) body.preferences = preferences

    const response = await httpClient(baseUrl(`${API_BASE}/start`), {
      method: 'POST',
      body: JSON.stringify(body),
    })
    return response.json
  },

  getState: async (sessionId) => {
    const response = await httpClient(
      baseUrl(`${API_BASE}/state?sessionId=${encodeURIComponent(sessionId)}`),
    )
    return response.json
  },

  next: async (sessionId, playedTrackId) => {
    const response = await httpClient(baseUrl(`${API_BASE}/next`), {
      method: 'POST',
      body: JSON.stringify({ sessionId, playedTrackId }),
    })
    return response.json
  },

  skip: async (sessionId, skippedTrackId) => {
    const response = await httpClient(baseUrl(`${API_BASE}/skip`), {
      method: 'POST',
      body: JSON.stringify({ sessionId, skippedTrackId }),
    })
    return response.json
  },

  endSession: async (sessionId) => {
    await httpClient(baseUrl(`${API_BASE}/end`), {
      method: 'POST',
      body: JSON.stringify({ sessionId }),
    })
  },

  // Interpret natural language prompt into structured preferences
  interpretPrompt: async (prompt) => {
    const response = await httpClient(baseUrl(`${API_BASE}/interpret-prompt`), {
      method: 'POST',
      body: JSON.stringify({ prompt }),
    })
    return response.json
  },

  // Fetch TTS audio for the provided commentary text
  getCommentaryAudio: async (text) => {
    return fetchAudio(
      `${API_BASE}/commentary-audio?text=${encodeURIComponent(text)}`,
    )
  },
}

export default aiDjApi
