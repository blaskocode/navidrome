import {
  AIDJ_DIALOG_OPEN,
  AIDJ_DIALOG_CLOSE,
  AIDJ_START_SESSION,
  AIDJ_START_SESSION_SUCCESS,
  AIDJ_START_SESSION_FAILURE,
  AIDJ_END_SESSION,
  AIDJ_END_SESSION_SUCCESS,
  AIDJ_END_SESSION_FAILURE,
  AIDJ_NEXT_TRACK,
  AIDJ_NEXT_TRACK_SUCCESS,
  AIDJ_NEXT_TRACK_FAILURE,
  AIDJ_SKIP_TRACK,
  AIDJ_SKIP_TRACK_SUCCESS,
  AIDJ_SKIP_TRACK_FAILURE,
  AIDJ_UPDATE_STATE,
  AIDJ_CLEAR_ERROR,
  AIDJ_SAVE_PLAY_MODE,
  AIDJ_SET_PREFERENCES,
  AIDJ_SPEECH_START,
  AIDJ_SPEECH_END,
  AIDJ_SPEECH_ERROR,
} from '../actions'

const initialState = {
  dialogOpen: false,
  seedTrackId: null,
  seedArtistId: null,
  active: false,
  loading: false,
  sessionId: null,
  mode: null,
  upNext: [],
  sets: [], // Pre-loaded sets with theme metadata
  chapterText: '',
  error: null,
  previousPlayMode: null,
  preferences: {
    energy: null, // null = any mood
    decade: null, // null = any decade
    contexts: [], // empty = no context filters
  },
  isSpeaking: false, // true while DJ commentary audio is playing
}

export const aiDjReducer = (state = initialState, action) => {
  switch (action.type) {
    case AIDJ_DIALOG_OPEN:
      return {
        ...state,
        dialogOpen: true,
        seedTrackId: action.seedTrackId,
        seedArtistId: action.seedArtistId,
      }

    case AIDJ_DIALOG_CLOSE:
      return {
        ...state,
        dialogOpen: false,
        seedTrackId: null,
        seedArtistId: null,
      }

    case AIDJ_START_SESSION:
      return {
        ...state,
        loading: true,
        error: null,
      }

    case AIDJ_START_SESSION_SUCCESS:
      return {
        ...state,
        dialogOpen: false,
        active: true,
        loading: false,
        sessionId: action.payload.sessionId,
        mode: action.payload.mode,
        upNext: action.payload.upNext || [],
        sets: action.payload.sets || [],
        chapterText: action.payload.chapterText || '',
        error: null,
      }

    case AIDJ_START_SESSION_FAILURE:
      return {
        ...state,
        loading: false,
        error: action.payload,
      }

    case AIDJ_END_SESSION:
      return {
        ...state,
        loading: true,
      }

    case AIDJ_END_SESSION_SUCCESS:
      return {
        ...initialState,
        isSpeaking: false,
      }

    case AIDJ_END_SESSION_FAILURE:
      return {
        ...state,
        loading: false,
        error: action.payload,
      }

    case AIDJ_NEXT_TRACK:
    case AIDJ_SKIP_TRACK:
      return {
        ...state,
        loading: true,
      }

    case AIDJ_NEXT_TRACK_SUCCESS:
    case AIDJ_SKIP_TRACK_SUCCESS:
      return {
        ...state,
        loading: false,
        upNext: action.payload.upNext || [],
        sets: action.payload.sets || state.sets,
        chapterText: action.payload.chapterText || state.chapterText,
        error: null,
      }

    case AIDJ_NEXT_TRACK_FAILURE:
    case AIDJ_SKIP_TRACK_FAILURE:
      return {
        ...state,
        loading: false,
        error: action.payload,
      }

    case AIDJ_UPDATE_STATE:
      return {
        ...state,
        ...action.payload,
      }

    case AIDJ_CLEAR_ERROR:
      return {
        ...state,
        error: null,
      }

    case AIDJ_SAVE_PLAY_MODE:
      return {
        ...state,
        previousPlayMode: action.payload,
      }

    case AIDJ_SET_PREFERENCES:
      return {
        ...state,
        preferences: action.preferences,
      }

    case AIDJ_SPEECH_START:
      return {
        ...state,
        isSpeaking: true,
      }

    case AIDJ_SPEECH_END:
    case AIDJ_SPEECH_ERROR:
      return {
        ...state,
        isSpeaking: false,
      }

    default:
      return state
  }
}
