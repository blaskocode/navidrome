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
  chapterText: '',
  error: null,
  previousPlayMode: null,
  preferences: {
    energy: null,     // null = any mood
    decade: null,     // null = any decade
    contexts: [],     // empty = no context filters
  },
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

    default:
      return state
  }
}
