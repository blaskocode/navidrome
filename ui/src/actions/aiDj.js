// AI DJ Action Types
export const AIDJ_DIALOG_OPEN = 'AIDJ_DIALOG_OPEN'
export const AIDJ_DIALOG_CLOSE = 'AIDJ_DIALOG_CLOSE'

export const AIDJ_START_SESSION = 'AIDJ_START_SESSION'
export const AIDJ_START_SESSION_SUCCESS = 'AIDJ_START_SESSION_SUCCESS'
export const AIDJ_START_SESSION_FAILURE = 'AIDJ_START_SESSION_FAILURE'

export const AIDJ_END_SESSION = 'AIDJ_END_SESSION'
export const AIDJ_END_SESSION_SUCCESS = 'AIDJ_END_SESSION_SUCCESS'
export const AIDJ_END_SESSION_FAILURE = 'AIDJ_END_SESSION_FAILURE'

export const AIDJ_NEXT_TRACK = 'AIDJ_NEXT_TRACK'
export const AIDJ_NEXT_TRACK_SUCCESS = 'AIDJ_NEXT_TRACK_SUCCESS'
export const AIDJ_NEXT_TRACK_FAILURE = 'AIDJ_NEXT_TRACK_FAILURE'

export const AIDJ_SKIP_TRACK = 'AIDJ_SKIP_TRACK'
export const AIDJ_SKIP_TRACK_SUCCESS = 'AIDJ_SKIP_TRACK_SUCCESS'
export const AIDJ_SKIP_TRACK_FAILURE = 'AIDJ_SKIP_TRACK_FAILURE'

export const AIDJ_UPDATE_STATE = 'AIDJ_UPDATE_STATE'
export const AIDJ_CLEAR_ERROR = 'AIDJ_CLEAR_ERROR'
export const AIDJ_SAVE_PLAY_MODE = 'AIDJ_SAVE_PLAY_MODE'

// Dialog Actions
export const openAiDjDialog = (seedTrackId = null, seedArtistId = null) => ({
  type: AIDJ_DIALOG_OPEN,
  seedTrackId,
  seedArtistId,
})

export const closeAiDjDialog = () => ({
  type: AIDJ_DIALOG_CLOSE,
})

// Action Creators
export const startSession = (
  mode,
  seedTrackId,
  seedArtistId,
  seedPlaylistId,
) => ({
  type: AIDJ_START_SESSION,
  payload: { mode, seedTrackId, seedArtistId, seedPlaylistId },
})

export const startSessionSuccess = (state) => ({
  type: AIDJ_START_SESSION_SUCCESS,
  payload: state,
})

export const startSessionFailure = (error) => ({
  type: AIDJ_START_SESSION_FAILURE,
  payload: error,
})

export const endSession = (sessionId) => ({
  type: AIDJ_END_SESSION,
  payload: { sessionId },
})

export const endSessionSuccess = () => ({
  type: AIDJ_END_SESSION_SUCCESS,
})

export const endSessionFailure = (error) => ({
  type: AIDJ_END_SESSION_FAILURE,
  payload: error,
})

export const nextTrack = (sessionId, playedTrackId) => ({
  type: AIDJ_NEXT_TRACK,
  payload: { sessionId, playedTrackId },
})

export const nextTrackSuccess = (state) => ({
  type: AIDJ_NEXT_TRACK_SUCCESS,
  payload: state,
})

export const nextTrackFailure = (error) => ({
  type: AIDJ_NEXT_TRACK_FAILURE,
  payload: error,
})

export const skipTrack = (sessionId, skippedTrackId) => ({
  type: AIDJ_SKIP_TRACK,
  payload: { sessionId, skippedTrackId },
})

export const skipTrackSuccess = (state) => ({
  type: AIDJ_SKIP_TRACK_SUCCESS,
  payload: state,
})

export const skipTrackFailure = (error) => ({
  type: AIDJ_SKIP_TRACK_FAILURE,
  payload: error,
})

export const updateAiDjState = (state) => ({
  type: AIDJ_UPDATE_STATE,
  payload: state,
})

export const clearAiDjError = () => ({
  type: AIDJ_CLEAR_ERROR,
})

export const aiDjSavePlayMode = (mode) => ({
  type: AIDJ_SAVE_PLAY_MODE,
  payload: mode,
})
